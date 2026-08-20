// Package powerflow 电力系统潮流计算:Slack/PV/PQ 母线模型 + Ybus +
// 极坐标牛顿-拉夫逊迭代,输出母线电压/注入功率/线路潮流/网损。
// 全部采用标幺值(pu),基值由 Input.BaseMVA 记录(默认 100MVA)。
package powerflow

import (
	"fmt"
	"sort"
)

// BusType 母线类型
type BusType int

const (
	BusSlack BusType = iota // 平衡母线:V、θ 给定,提供功率缺口
	BusPV                   // 发电机母线:V、P 给定,Q 可调(限幅)
	BusPQ                   // 负荷母线:P、Q 给定,求 V、θ
)

// String 母线类型名
func (b BusType) String() string {
	switch b {
	case BusSlack:
		return "slack"
	case BusPV:
		return "pv"
	default:
		return "pq"
	}
}

// Bus 母线定义(标幺值)
type Bus struct {
	ID     int      // 母线编号(任意整数,输出以该 ID 为键)
	Type   BusType  // 母线类型
	V      float64  // 电压幅值(Slack/PV 给定;PQ 为初值,默认 1.0)
	Theta  float64  // 电压相角(弧度;Slack 给定,其余初值 0)
	P, Q   float64  // 注入功率:PV 给 P;PQ 给 P,Q;Slack 不给(求解)
	Qmin   float64  // PV 无功下限(默认 -∞,即不限制)
	Qmax   float64  // PV 无功上限(默认 +∞,即不限制)
}

// Branch 支路定义(标幺值)
type Branch struct {
	From, To int     // 两端母线 ID
	R, X     float64 // 串联阻抗 z = R + jX(必须 > 0)
	B        float64 // 对地电纳(可选,默认 0)
	Tap      float64 // 变压器变比(可选,默认 1;1 = 普通线路)
}

// Input 潮流计算输入
type Input struct {
	Frequency float64 // 频率(Hz,仅记录)
	BaseMVA   float64 // 标幺基值(仅记录,默认 100)
	Buses     []Bus
	Branches  []Branch
}

// Options 求解选项
type Options struct {
	Tol       float64 // 失配量收敛阈值(标幺,默认 1e-6)
	MaxIter   int     // 最大迭代次数(默认 50)
	FlatStart bool    // 平启动:PQ 母线 V=1.0∠0°(默认 true)
	Damping   float64 // 阻尼因子(默认 1.0;发散时可调小)
}

// BranchFlow 支路潮流(标幺)
type BranchFlow struct {
	From, To    int
	PFrom, QFrom float64 // From 端送出功率
	PTo, QTo     float64 // To 端送出功率
}

// Loss 支路损耗
func (b BranchFlow) Loss() (p, q float64) {
	return b.PFrom + b.PTo, b.QFrom + b.QTo
}

// Result 潮流计算结果
type Result struct {
	Converged   bool
	Iterations  int
	BusV        map[int]complex128 // 母线 ID → 电压相量 V∠θ
	GenPower    map[int]complex128 // 母线 ID → 注入 S=P+jQ(Slack/PV)
	BranchFlow  map[string]BranchFlow // "From->To" → 支路潮流
	TotalLoss   complex128          // 全网网损(标幺)
	IterationLog []float64          // 每轮最大失配量(调试/收敛曲线)
}

// solveBus 内部母线:索引 + ID
type solveBus struct {
	Bus
	idx      int     // 内部索引(按 ID 排序)
	origType BusType // 输入时的类型(PV 转 PQ 后保留,注入功率仍按其输出)
}

// prepare 校验输入并构建内部结构
func prepare(in Input, opts *Options) (*solver, error) {
	if opts == nil {
		opts = &Options{}
	}
	if opts.Tol <= 0 {
		opts.Tol = 1e-6
	}
	if opts.MaxIter <= 0 {
		opts.MaxIter = 50
	}
	if opts.Damping <= 0 || opts.Damping > 1 {
		opts.Damping = 1.0
	}
	if len(in.Buses) < 2 {
		return nil, fmt.Errorf("潮流计算至少需要 2 条母线")
	}
	slackCount := 0
	seen := make(map[int]bool)
	for i := range in.Buses {
		if seen[in.Buses[i].ID] {
			return nil, fmt.Errorf("母线 ID %d 重复", in.Buses[i].ID)
		}
		seen[in.Buses[i].ID] = true
		if in.Buses[i].Type == BusSlack {
			slackCount++
		}
	}
	if slackCount != 1 {
		return nil, fmt.Errorf("潮流计算必须恰好 1 条 Slack 母线(实际 %d 条)", slackCount)
	}
	// 按 ID 排序建立内部索引
	buses := make([]solveBus, len(in.Buses))
	ids := make([]int, len(in.Buses))
	for i, b := range in.Buses {
		ids[i] = b.ID
	}
	sort.Ints(ids)
	idToIdx := make(map[int]int, len(ids))
	for i, id := range ids {
		idToIdx[id] = i
	}
	for i, b := range in.Buses {
		sb := solveBus{Bus: b, idx: idToIdx[b.ID], origType: b.Type}
		// PQ 母线平启动:FlatStart(默认)或未给初值(V<=0)时 V=1.0
		if sb.Type == BusPQ && (opts.FlatStart || sb.V <= 0) {
			sb.V = 1.0
		}
		if sb.Type != BusPQ && sb.V <= 0 {
			sb.V = 1.0
		}
		buses[i] = sb
	}
	// 校验支路
	for i, br := range in.Branches {
		if br.R < 0 || br.X < 0 || (br.R == 0 && br.X == 0) {
			return nil, fmt.Errorf("支路 %d(%d->%d) 阻抗非法: R=%g X=%g", i, br.From, br.To, br.R, br.X)
		}
		if _, ok := idToIdx[br.From]; !ok {
			return nil, fmt.Errorf("支路 %d 端点 %d 不是母线", i, br.From)
		}
		if _, ok := idToIdx[br.To]; !ok {
			return nil, fmt.Errorf("支路 %d 端点 %d 不是母线", i, br.To)
		}
	}
	s := &solver{
		buses:    buses,
		branches: in.Branches,
		idToIdx:  idToIdx,
		opts:     opts,
	}
	s.buildYbus()
	return s, nil
}
