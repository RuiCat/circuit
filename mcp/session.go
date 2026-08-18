package mcp

import (
	"circuit/element"
	"circuit/load"
	"circuit/mna"
	"fmt"
	"sync"
	"time"
)

// SimConfig 仿真参数配置。
type SimConfig struct {
	InitialStep float64 `json:"initialStep"` // 初始步长（秒）
	MinStep     float64 `json:"minStep"`     // 最小步长（秒）
	MaxStep     float64 `json:"maxStep"`     // 最大步长（秒）
	AbsTol      float64 `json:"absTol"`      // 绝对容差
	RelTol      float64 `json:"relTol"`      // 相对容差
	MaxIter     int     `json:"maxIter"`     // 最大非线性迭代次数
	MaxSteps    int     `json:"maxSteps"`    // 最大时间步数
	Parallel    int     `json:"parallel"`    // 并行 worker 数（0=串行）
}

// DefaultSimConfig 返回默认仿真参数（与 CLI 默认一致）。
func DefaultSimConfig() SimConfig {
	return SimConfig{
		InitialStep: 1e-6,
		MinStep:     1e-9,
		MaxStep:     5e-4,
		AbsTol:      1e-6,
		RelTol:      1e-4,
		MaxIter:     200,
		MaxSteps:    10000,
		Parallel:    0,
	}
}

// Session 仿真会话：一个网表上下文 + 仿真参数 + 任务状态。
// 同一会话同时最多一个运行中的任务（mu 串行化）。
type Session struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Netlist   string           `json:"netlist"`
	Con       *element.Context `json:"-"`
	SimCfg    SimConfig        `json:"simCfg"`
	CreatedAt time.Time        `json:"createdAt"`
	LastUsed  time.Time        `json:"lastUsed"`

	mu       sync.Mutex
	job      *Job          // 当前/最近任务
	jobSeq   uint64        // 任务序号
	history  []*Job        // 任务历史（环形，上限 historyCap）
	triggers []mna.Trigger // 会话触发点（任务启动时应用到新的时间控制器）
}

const historyCap = 16

// SessionInfo 会话列表项。
type SessionInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Elements  int       `json:"elements"`
	Nodes     int       `json:"nodes"`
	HasJob    bool      `json:"hasJob"`
	JobID     string    `json:"jobId,omitempty"`
	JobState  string    `json:"jobState,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	LastUsed  time.Time `json:"lastUsed"`
}

// SessionStore 会话存储（并发安全）。
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	seq      uint64
	TTL      time.Duration // 空闲自动清理 TTL，0=不清理
}

// NewSessionStore 创建会话存储。
func NewSessionStore(ttl time.Duration) *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*Session),
		TTL:      ttl,
	}
}

// New 创建新会话（可选初始网表）。
func (s *SessionStore) New(name, netlist string) (*Session, error) {
	s.mu.Lock()
	s.seq++
	id := fmt.Sprintf("s%d", s.seq)
	s.mu.Unlock()

	sess := &Session{
		ID:        id,
		Name:      name,
		SimCfg:    DefaultSimConfig(),
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
	}
	if netlist != "" {
		if err := sess.LoadNetlist(netlist); err != nil {
			return nil, err
		}
	}
	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()
	return sess, nil
}

// Get 获取会话。
func (s *SessionStore) Get(id string) (*Session, error) {
	s.mu.RLock()
	sess, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("会话 %s 不存在（可用 circuit_list_sessions 查看）", id)
	}
	sess.mu.Lock()
	sess.LastUsed = time.Now()
	sess.mu.Unlock()
	return sess, nil
}

// Delete 删除会话；若有运行中任务先取消。
func (s *SessionStore) Delete(id string) error {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	if ok {
		delete(s.sessions, id)
	}
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("会话 %s 不存在", id)
	}
	if j := sess.CurrentJob(); j != nil {
		j.Cancel()
	}
	return nil
}

// List 列出全部会话。
func (s *SessionStore) List() []SessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SessionInfo, 0, len(s.sessions))
	for _, sess := range s.sessions {
		info := SessionInfo{
			ID:        sess.ID,
			Name:      sess.Name,
			CreatedAt: sess.CreatedAt,
		}
		sess.mu.Lock()
		info.LastUsed = sess.LastUsed
		sess.mu.Unlock()
		if sess.Con != nil {
			info.Elements = len(sess.Con.Nodelist)
			info.Nodes = len(sess.Con.CompactNodeID)
		}
		if j := sess.CurrentJob(); j != nil {
			info.HasJob = true
			info.JobID = j.ID
			info.JobState = string(j.StateStr())
		}
		out = append(out, info)
	}
	return out
}

// CleanupLoop 定期清理空闲超时会话（TTL<=0 时不做任何事）。
func (s *SessionStore) CleanupLoop(stop <-chan struct{}) {
	if s.TTL <= 0 {
		<-stop
		return
	}
	ticker := time.NewTicker(s.TTL / 2)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-s.TTL)
			s.mu.Lock()
			var expired []string
			for id, sess := range s.sessions {
				sess.mu.Lock()
				idle := sess.LastUsed.Before(cutoff)
				sess.mu.Unlock()
				if idle {
					expired = append(expired, id)
				}
			}
			for _, id := range expired {
				delete(s.sessions, id)
			}
			s.mu.Unlock()
		}
	}
}

// LoadNetlist 加载/替换会话网表。失败时保留旧网表与旧上下文。
func (s *Session) LoadNetlist(netlist string) error {
	con, err := load.LoadString(netlist)
	if err != nil {
		return err
	}
	con.InitResumeCond()

	s.mu.Lock()
	defer s.mu.Unlock()
	if j := s.job; j != nil && j.State == JobRunning {
		return fmt.Errorf("会话 %s 有运行中的任务 %s，请先取消或等待完成", s.ID, j.ID)
	}
	old := s.Con
	s.Netlist = netlist
	s.Con = con
	s.job = nil
	s.history = nil
	if old != nil {
		// 显式置空，允许 GC 回收旧上下文
		old = nil
	}
	return nil
}

// CurrentJob 返回当前（最近）任务。
func (s *Session) CurrentJob() *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.job
}

// RunningJob 返回运行中的任务（无则 nil）。
func (s *Session) RunningJob() *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job != nil && s.job.IsRunning() {
		return s.job
	}
	return nil
}

// GetJob 按 ID 查找任务（当前或历史）。
func (s *Session) GetJob(id string) *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job != nil && s.job.ID == id {
		return s.job
	}
	for _, j := range s.history {
		if j.ID == id {
			return j
		}
	}
	return nil
}

// pushJob 记录任务到历史（保留最近 historyCap 个）。
func (s *Session) pushJob(j *Job) {
	s.history = append(s.history, j)
	if len(s.history) > historyCap {
		s.history = s.history[len(s.history)-historyCap:]
	}
}

// SetEvent 设置事件值（引擎保证 goroutine 安全，运行中任务也可调用）。
func (s *Session) SetEvent(name string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Con == nil {
		return fmt.Errorf("会话 %s 尚未加载网表", s.ID)
	}
	s.Con.SetEvent(name, value)
	return nil
}
