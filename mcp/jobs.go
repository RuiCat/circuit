package mcp

import (
	"circuit/element"
	etime "circuit/element/time"
	"circuit/mna"
	"fmt"
	"sort"
	"sync"
	"time"
)

// JobState 任务状态。
type JobState string

const (
	JobPending   JobState = "pending"
	JobRunning   JobState = "running"
	JobDone      JobState = "done"
	JobError     JobState = "error"
	JobCancelled JobState = "cancelled"
)

// maxRawPoints 原始采样点上限（超出后自动减半分辨率）。
const maxRawPoints = 262144

// Probe 结果探针：一个输出列。
type Probe struct {
	Name string // 列名（如 "node_1"、"I(R1)"）
	Kind string // "node" | "current"
	Unit string // 单位
	// read 从每步电压数组读取该列值（v 前 nodesNum 项为节点电压，之后为电压源电流）。
	read func(v []float64) float64
}

// Job 异步仿真任务。
type Job struct {
	ID          string         `json:"id"`
	SessionID   string         `json:"sessionId"`
	Kind        string         `json:"kind"` // "transient" | "dc"
	State       JobState       `json:"state"`
	Error       string         `json:"error,omitempty"`
	Steps       int64          `json:"steps"`
	CurrentTime float64        `json:"currentTime"`
	FinalTime   float64        `json:"finalTime"`
	Probes      []ProbeSummary `json:"probes"`
	StartedAt   time.Time      `json:"startedAt"`
	FinishedAt  time.Time      `json:"finishedAt,omitempty"`

	con       *element.Context
	sess      *Session
	probes    []Probe
	curSlots  []curSlot // 全部电流槽描述（快照用）
	times     []float64
	series    map[string][]float64
	snapV     map[string]float64 // 原始节点 ID → 电压（逐步快照，仿真协程内录制）
	snapI     map[string]float64 // 电流列名 → 值（逐步快照，仿真协程内录制）
	finalV    map[string]float64 // 原始节点 ID → 电压（任务结束时快照）
	finalI    map[string]float64 // 电流列名 → 值（任务结束时快照）
	cancelled bool

	mu   sync.Mutex
	done chan struct{} // 任务终结时关闭
}

// curSlot 电流槽描述（电压源支路电流 + 元件自身电流槽）。
type curSlot struct {
	Name   string // 列名（如 "I(R1)"）
	Unit   string
	elem   element.NodeFace
	vsID   int // >=0: 电压源支路电流（读电压数组尾部）
	idx    int // >=0: 元件自身电流槽（读 NodeValue）
	offset int // 电压源电流在电压数组中的偏移（nodesNum+vsID）
}

// read 读取当前值。v 为最近一步的电压数组（可为 nil，此时仅元件槽可读）。
func (cs curSlot) read(v []float64) float64 {
	if cs.vsID >= 0 {
		if v != nil && cs.offset >= 0 && cs.offset < len(v) {
			return v[cs.offset]
		}
		return 0
	}
	return cs.elem.GetFloat64(cs.idx)
} // buildCurrentSlots 收集上下文中全部电流槽（与 elementCurrents 同名同序）。
func buildCurrentSlots(con *element.Context) []curSlot {
	nodesNum := con.GetNodeNum()
	var out []curSlot
	for _, elem := range con.Nodelist {
		cfg := elem.Config()
		inst := instanceName(elem)
		unit := unitOf(cfg)
		for vs := 0; vs < cfg.VoltageNum(); vs++ {
			vsID := elem.GetVoltSource(vs)
			name := fmt.Sprintf("I(%s)", inst)
			if cfg.VoltageNum() > 1 {
				name = fmt.Sprintf("I(%s[%d])", inst, vs)
			}
			out = append(out, curSlot{
				Name:   name,
				Unit:   unit,
				elem:   elem,
				vsID:   int(vsID),
				offset: nodesNum + int(vsID),
			})
		}
		for _, idx := range cfg.Current {
			if idx < 0 || idx >= cfg.ValueNum() {
				continue
			}
			switch cfg.ValueInit[idx].(type) {
			case float64, nil:
			default:
				continue
			}
			name := fmt.Sprintf("I(%s)", inst)
			if len(cfg.Current) > 1 {
				suffix := cfg.ValueName[idx]
				if suffix == "" {
					suffix = fmt.Sprintf("i%d", idx)
				}
				name = fmt.Sprintf("I(%s.%s)", inst, suffix)
			}
			out = append(out, curSlot{Name: name, Unit: unit, elem: elem, vsID: -1, idx: idx})
		}
	}
	return out
}

// ProbeSummary 结果列摘要。
type ProbeSummary struct {
	Name  string  `json:"name"`
	Kind  string  `json:"kind"`
	Unit  string  `json:"unit"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Last  float64 `json:"last"`
	First float64 `json:"first"`
}

// NewJob 创建任务（不启动）。
func NewJob(sess *Session, kind string) *Job {
	sess.mu.Lock()
	sess.jobSeq++
	seq := sess.jobSeq
	sess.mu.Unlock()
	return &Job{
		ID:        fmt.Sprintf("%s-j%d", sess.ID, seq),
		SessionID: sess.ID,
		Kind:      kind,
		State:     JobPending,
		sess:      sess,
		series:    make(map[string][]float64),
		snapV:     make(map[string]float64),
		snapI:     make(map[string]float64),
		finalV:    make(map[string]float64),
		finalI:    make(map[string]float64),
		done:      make(chan struct{}),
	}
}

// IsRunning 任务是否运行中。
func (j *Job) IsRunning() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.State == JobRunning
}

// StateStr 返回当前状态（并发安全）。
func (j *Job) StateStr() JobState {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.State
}

// Snapshot 返回最近一步的节点电压与电流快照（并发安全，可在任务运行中调用）。
func (j *Job) Snapshot() (map[string]float64, map[string]float64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	v := make(map[string]float64, len(j.snapV))
	for k, val := range j.snapV {
		v[k] = val
	}
	i := make(map[string]float64, len(j.snapI))
	for k, val := range j.snapI {
		i[k] = val
	}
	return v, i
}

// Cancel 请求取消任务（引擎 Stop 优雅退出，非强制）。
func (j *Job) Cancel() {
	j.mu.Lock()
	j.cancelled = true
	con := j.con
	j.mu.Unlock()
	if con != nil {
		if tm, ok := con.Time.(*etime.TimeMNA); ok {
			tm.Stop()
		}
	}
}

// Wait 等待任务终结，超时返回 false。
func (j *Job) Wait(timeout time.Duration) bool {
	select {
	case <-j.done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// finalize 任务终结：关闭 done 通道。
func (j *Job) finalize() {
	close(j.done)
}

// record 记录一步采样（超出上限时减半分辨率），并刷新节点/电流快照。
// 仅在仿真协程内调用（与引擎写操作同 goroutine，无需额外同步）。
func (j *Job) record(con *element.Context, v []float64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	t := con.CurrentTime()
	if len(j.times) >= maxRawPoints {
		j.halve()
	}
	j.times = append(j.times, t)
	for _, p := range j.probes {
		j.series[p.Name] = append(j.series[p.Name], p.read(v))
	}
	// 快照：全部节点电压 + 全部电流槽
	for rawID, compactIdx := range con.CompactNodeID {
		if compactIdx >= 0 && compactIdx < len(v) {
			j.snapV[fmt.Sprintf("node_%d", int(rawID))] = v[compactIdx]
		}
	}
	for _, cs := range j.curSlots {
		j.snapI[cs.Name] = cs.read(v)
	}
	j.Steps++
	j.CurrentTime = t
}

// halve 减半采样分辨率（保留偶数索引样本）。
func (j *Job) halve() {
	n := len(j.times) / 2
	nt := make([]float64, 0, n)
	for i := 0; i < len(j.times); i += 2 {
		nt = append(nt, j.times[i])
	}
	j.times = nt
	for name, s := range j.series {
		ns := make([]float64, 0, n)
		for i := 0; i < len(s); i += 2 {
			ns = append(ns, s[i])
		}
		j.series[name] = ns
	}
}

// buildProbes 构建探针列表：节点（原始 ID）+ 元件电流（实例名展开）。
// 未指定时默认输出全部节点电压。
func buildProbes(con *element.Context, nodeIDs []int, instances []string) ([]Probe, error) {
	var probes []Probe
	nodesNum := con.GetNodeNum()

	nodeUnit := make(map[int]string)
	nodePinType := make(map[int]element.PinType)
	for _, elem := range con.Nodelist {
		cfg := elem.Config()
		for i := range cfg.Pin {
			compactIdx := int(elem.GetNodes(i))
			if compactIdx < 0 {
				continue
			}
			if _, ok := nodePinType[compactIdx]; !ok {
				nodePinType[compactIdx] = cfg.Pin[i].Type
			}
		}
	}
	for compactIdx, pt := range nodePinType {
		nodeUnit[compactIdx] = nodeUnitOf(pt)
	}

	addNode := func(rawID int) error {
		compactIdx, ok := con.CompactNodeID[mna.NodeID(rawID)]
		if !ok {
			return fmt.Errorf("节点 %d 不在网表中", rawID)
		}
		unit := "V"
		if u, ok := nodeUnit[compactIdx]; ok {
			unit = u
		}
		probes = append(probes, Probe{
			Name: fmt.Sprintf("node_%d", rawID),
			Kind: "node",
			Unit: unit,
			read: func(v []float64) float64 {
				if compactIdx < len(v) {
					return v[compactIdx]
				}
				return 0
			},
		})
		return nil
	}

	addElement := func(inst string) error {
		var elem element.NodeFace
		for _, e := range con.Nodelist {
			if instanceName(e) == inst {
				elem = e
				break
			}
		}
		if elem == nil {
			return fmt.Errorf("元件 %s 不在网表中", inst)
		}
		cfg := elem.Config()
		unit := unitOf(cfg)
		// 电压源支路电流：位于电压数组尾部
		for vs := 0; vs < cfg.VoltageNum(); vs++ {
			vsID := elem.GetVoltSource(vs)
			name := fmt.Sprintf("I(%s)", inst)
			if cfg.VoltageNum() > 1 {
				name = fmt.Sprintf("I(%s[%d])", inst, vs)
			}
			probes = append(probes, Probe{
				Name: name,
				Kind: "current",
				Unit: unit,
				read: func(v []float64) float64 {
					idx := nodesNum + int(vsID)
					if idx >= 0 && idx < len(v) {
						return v[idx]
					}
					return 0
				},
			})
		}
		// 元件自身电流槽：仿真中由元件写入，运行时读取
		for _, idx := range cfg.Current {
			if idx < 0 || idx >= cfg.ValueNum() {
				continue
			}
			switch cfg.ValueInit[idx].(type) {
			case float64, nil:
			default:
				continue
			}
			name := fmt.Sprintf("I(%s)", inst)
			if len(cfg.Current) > 1 {
				suffix := cfg.ValueName[idx]
				if suffix == "" {
					suffix = fmt.Sprintf("i%d", idx)
				}
				name = fmt.Sprintf("I(%s.%s)", inst, suffix)
			}
			probes = append(probes, Probe{
				Name: name,
				Kind: "current",
				Unit: unit,
				read: func(v []float64) float64 {
					return elem.GetFloat64(idx)
				},
			})
		}
		return nil
	}

	if len(nodeIDs) == 0 && len(instances) == 0 {
		// 默认：全部节点
		rawIDs := make([]int, 0, len(con.CompactNodeID))
		for rawID := range con.CompactNodeID {
			rawIDs = append(rawIDs, int(rawID))
		}
		sort.Ints(rawIDs)
		for _, rawID := range rawIDs {
			if err := addNode(rawID); err != nil {
				return nil, err
			}
		}
		return probes, nil
	}
	for _, rawID := range nodeIDs {
		if err := addNode(rawID); err != nil {
			return nil, err
		}
	}
	for _, inst := range instances {
		if err := addElement(inst); err != nil {
			return nil, err
		}
	}
	return probes, nil
}

// RunTransient 启动瞬态仿真任务（异步）。targetTime<=0 时使用配置的初始步长。
func (s *Session) RunTransient(targetTime float64, nodeIDs []int, instances []string, cfg *SimConfig) (*Job, error) {
	s.mu.Lock()
	if s.Con == nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("会话 %s 尚未加载网表", s.ID)
	}
	if j := s.job; j != nil && j.IsRunning() {
		s.mu.Unlock()
		return nil, fmt.Errorf("会话 %s 已有运行中的任务 %s", s.ID, j.ID)
	}
	con := s.Con
	s.mu.Unlock()

	if cfg == nil {
		s.mu.Lock()
		c := s.SimCfg
		s.mu.Unlock()
		cfg = &c
	}
	if targetTime <= 0 {
		targetTime = cfg.InitialStep
	}
	probes, err := buildProbes(con, nodeIDs, instances)
	if err != nil {
		return nil, err
	}
	job := NewJob(s, "transient")
	job.probes = probes
	for _, p := range probes {
		job.Probes = append(job.Probes, ProbeSummary{Name: p.Name, Kind: p.Kind, Unit: p.Unit})
	}
	job.curSlots = buildCurrentSlots(con)

	// 构建时间控制器（与 CLI 批处理流程一致）
	tm, err := etime.NewTimeMNA(targetTime)
	if err != nil {
		return nil, fmt.Errorf("创建时间控制器失败: %w", err)
	}
	s.mu.Lock()
	triggers := append([]mna.Trigger(nil), s.triggers...)
	con.Time = tm // 在会话锁内赋值，避免与其他工具的 Time 读取竞争
	s.mu.Unlock()
	if len(triggers) > 0 {
		tm.SetTriggers(triggers)
	}
	con.InitResumeCond()
	tm.SetNotifier(func() {
		if c := con.ResumeCond(); c != nil {
			c.Broadcast()
		}
	})
	if err := con.Time.SetTolerances(cfg.AbsTol, cfg.RelTol); err != nil {
		return nil, fmt.Errorf("设置容差失败: %w", err)
	}
	if err := con.Time.SetStepLimits(cfg.MinStep, cfg.MaxStep); err != nil {
		return nil, fmt.Errorf("设置步长限制失败: %w", err)
	}
	if err := con.Time.SetIterationLimits(cfg.MaxIter, tm.MaxElemIter()); err != nil {
		return nil, fmt.Errorf("设置迭代限制失败: %w", err)
	}
	if err := con.Time.SetMaxTimeSteps(cfg.MaxSteps); err != nil {
		return nil, fmt.Errorf("设置最大步数失败: %w", err)
	}
	con.Time.SetTimeStep(cfg.InitialStep)
	if cfg.Parallel > 1 {
		con.ParallelOpts = &element.ParallelOptions{StampWorkers: cfg.Parallel}
	} else {
		con.ParallelOpts = nil
	}

	job.con = con
	s.mu.Lock()
	// 原子检查+赋值，防止两个并发 RunTransient 同时通过检查；
	// 若期间网表被替换（LoadNetlist），放弃本次启动
	if j := s.job; j != nil && j.IsRunning() {
		s.mu.Unlock()
		return nil, fmt.Errorf("会话 %s 已有运行中的任务 %s", s.ID, j.ID)
	}
	if s.Con != con {
		s.mu.Unlock()
		return nil, fmt.Errorf("会话 %s 网表已变更，请重试", s.ID)
	}
	s.job = job
	s.mu.Unlock()

	job.mu.Lock()
	job.State = JobRunning
	job.StartedAt = time.Now()
	job.mu.Unlock()

	go func() {
		err := etime.TransientSimulation(con, func(v []float64) {
			job.record(con, v)
		})
		job.finish(con, err)
	}()
	return job, nil
}

// RunDC 启动 DC 工作点分析任务：以极小目标时间运行瞬态仿真。
// 纯 DC 电路即时收敛到稳态解；含储能元件时返回 t=0 初始解。
func (s *Session) RunDC(nodeIDs []int, instances []string, cfg *SimConfig) (*Job, error) {
	if cfg == nil {
		s.mu.Lock()
		c := s.SimCfg
		s.mu.Unlock()
		cfg = &c
	}
	// DC 目标时间 = 初始步长，一步到位（运行检查与赋值在 RunTransient 内完成）
	return s.RunTransient(cfg.InitialStep, nodeIDs, instances, cfg)
}

// finish 任务终结处理：状态转换 + 终值快照。
func (j *Job) finish(con *element.Context, simErr error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.FinishedAt = time.Now()
	j.FinalTime = con.CurrentTime()

	// 终值快照：全部节点电压 + 全部电流槽
	for rawID := range con.CompactNodeID {
		j.finalV[fmt.Sprintf("node_%d", int(rawID))] = con.GetRawNodeVoltage(rawID)
	}
	for _, cs := range j.curSlots {
		j.finalI[cs.Name] = cs.read(nil)
	}
	// 快照与终值对齐（任务结束后快照即终值）
	for k, v := range j.finalV {
		j.snapV[k] = v
	}
	for k, v := range j.finalI {
		j.snapI[k] = v
	}
	// 探针摘要
	for i := range j.Probes {
		p := j.probes[i]
		s := j.series[p.Name]
		if len(s) == 0 {
			continue
		}
		min, max := s[0], s[0]
		for _, v := range s {
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
		j.Probes[i].First = s[0]
		j.Probes[i].Last = s[len(s)-1]
		j.Probes[i].Min = min
		j.Probes[i].Max = max
	}

	switch {
	case j.cancelled:
		j.State = JobCancelled
	case simErr != nil:
		j.State = JobError
		j.Error = simErr.Error()
	default:
		j.State = JobDone
	}
	// 记录到会话历史
	if j.sess != nil {
		j.sess.pushJob(j)
	}
	j.finalize()
}
