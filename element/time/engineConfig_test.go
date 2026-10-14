// 引擎配置对象（契约 K3）的回归网：后端绑定的载入期不可变语义、互斥校验与
// 实际生效后端解析；策略绑定字段映射。
//
// 关键语义（冻结）：
//   - 载入期由 CIRCUIT_LUSPARSE/CIRCUIT_LUSDENSE 显式绑定时，任何后端开关改动 → 报错，
//     文案含"载入期绑定"；
//   - 载入期未显式绑定时，只拒绝会改变已绑定**矩阵存储**的写入
//     （默认载入即稀疏存储 ⇒ forceDense=true 报错；sparseLU=true 是无操作，按原语义放行）；
//   - SparseLU && ForceDense 同时为真 → Validate 与 ResolveBackend 都报错。
package time

import (
	"circuit/element"
	"circuit/mna"
	"math"
	"strings"
	"testing"
)

// newBoundContext 构造一个已完成载入期绑定的最小 Context（复刻 load 的接线顺序）。
func newBoundContext(sparseLU, forceDense bool) *element.Context {
	con := &element.Context{}
	con.EngineCfg.SparseLU = sparseLU
	con.EngineCfg.ForceDense = forceDense
	con.Backend.Bind(sparseLU, forceDense)
	return con
}

// TestEngineConfigBackendBindingMutualExclusion 互斥校验：注释声称的互斥变成代码强制。
func TestEngineConfigBackendBindingMutualExclusion(t *testing.T) {
	both := BackendBinding{SparseLU: true, ForceDense: true}
	err := both.Validate()
	if err == nil {
		t.Fatal("SparseLU && ForceDense 必须报错，实际 nil")
	}
	if !strings.Contains(err.Error(), "互斥") {
		t.Fatalf("互斥错误文案应含\"互斥\"，实际 %q", err.Error())
	}
	if _, err := both.ResolveBackend(0.1, 1); err == nil {
		t.Fatal("ResolveBackend 遇互斥绑定必须报错，实际 nil")
	}
	for _, b := range []BackendBinding{{SparseLU: true}, {ForceDense: true}, {}} {
		if err := b.Validate(); err != nil {
			t.Fatalf("非互斥绑定不应报错: %+v -> %v", b, err)
		}
	}
}

// TestEngineConfigRuntimeBackendRewriteRejected 运行期改写后端 → 报错且文案含"载入期绑定"。
func TestEngineConfigRuntimeBackendRewriteRejected(t *testing.T) {
	// 情形一：载入期未显式绑定（自动，默认稀疏存储）。
	con := newBoundContext(false, false)
	if !con.Backend.Bound || !con.Backend.BoundSparse {
		t.Fatalf("载入期绑定记录错误: %+v", con.Backend)
	}
	rewrite := BackendBindingOf(con)
	rewrite.ForceDense = true // 稀疏 → 稠密：真实改变已绑定后端
	err := rewrite.Validate()
	if err == nil {
		t.Fatal("自动载入下改写存储后端（forceDense=true）必须报错")
	}
	if !strings.Contains(err.Error(), "载入期绑定") {
		t.Fatalf("错误文案必须含\"载入期绑定\"（机检依赖），实际 %q", err.Error())
	}
	// 无操作写入（sparseLU=true：稀疏 → 稀疏）按原语义放行。
	noop := BackendBindingOf(con)
	noop.SparseLU = true
	if err := noop.Validate(); err != nil {
		t.Fatalf("不改变已绑定存储的写入应放行，实际 %v", err)
	}

	// 情形二：载入期由环境变量显式绑定 → 任何后端开关改动都算改写。
	explicit := newBoundContext(true, false)
	if !explicit.Backend.Bound || !explicit.Backend.BoundSparse || !explicit.Backend.SparseLU {
		t.Fatalf("载入期显式绑定记录错误: %+v", explicit.Backend)
	}
	turnOff := BackendBindingOf(explicit)
	turnOff.SparseLU = false
	err = turnOff.Validate()
	if err == nil {
		t.Fatal("显式绑定下改写 sparseLU 必须报错")
	}
	if !strings.Contains(err.Error(), "载入期绑定") {
		t.Fatalf("错误文案必须含\"载入期绑定\"，实际 %q", err.Error())
	}
}

// TestEngineConfigBindBackendRecordsStorage 绑定记录与存储判据（SparseLU || !ForceDense）。
func TestEngineConfigBindBackendRecordsStorage(t *testing.T) {
	cases := []struct {
		sparseLU    bool
		forceDense  bool
		wantStorage bool
	}{
		{false, false, true},
		{true, false, true},
		{false, true, false},
		// 两个环境变量同时设置的既存组合：载入期不拒绝（本轮只登记），存储按既有判据取稀疏。
		{true, true, true},
	}
	for _, c := range cases {
		con := newBoundContext(c.sparseLU, c.forceDense)
		if !con.Backend.Bound {
			t.Fatalf("(%v,%v): Bound 应为 true", c.sparseLU, c.forceDense)
		}
		if con.Backend.BoundSparse != c.wantStorage {
			t.Fatalf("(%v,%v): BoundSparse 应为 %v，实际 %v", c.sparseLU, c.forceDense, c.wantStorage, con.Backend.BoundSparse)
		}
	}
}

// TestEngineConfigResolveBackend 实际生效后端的解析表：与 chooseLUSolver 逐分支一致。
func TestEngineConfigResolveBackend(t *testing.T) {
	cases := []struct {
		b       BackendBinding
		density float64
		workers int
		want    bool
		wantErr bool
	}{
		{BackendBinding{SparseLU: true}, 0.9, 1, true, false}, // 显式稀疏优先于密度
		{BackendBinding{SparseLU: true}, 0.1, 4, true, false}, // 显式稀疏优先于并行稠密（顺序保持）
		{BackendBinding{ForceDense: true}, 0.1, 1, false, false},
		{BackendBinding{}, 0.1, 1, true, false},  // 自动：稀疏矩阵
		{BackendBinding{}, 0.9, 1, false, false}, // 自动：稠密矩阵
		{BackendBinding{}, 0.1, 4, false, false}, // 并行（StampWorkers>1）→ 并行稠密
		{BackendBinding{}, 0, 0, true, false},    // 空矩阵退化为稀疏（与原 n==0 分支一致）
		{BackendBinding{SparseLU: true, ForceDense: true}, 0.1, 1, false, true},
		{BackendBinding{}, -1, 1, false, true},         // 非法密度
		{BackendBinding{}, math.NaN(), 1, false, true}, // NaN
	}
	for _, c := range cases {
		got, err := c.b.ResolveBackend(c.density, c.workers)
		if c.wantErr {
			if err == nil {
				t.Fatalf("%+v density=%v workers=%d 应报错", c.b, c.density, c.workers)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%+v density=%v workers=%d 不应报错: %v", c.b, c.density, c.workers, err)
		}
		if got != c.want {
			t.Fatalf("%+v density=%v workers=%d 稀疏标志应为 %v，实际 %v", c.b, c.density, c.workers, c.want, got)
		}
	}
}

// TestEngineConfigChooseLUSolverHonorsBinding 引擎路径确实走 ResolveBackend：
// 互斥绑定在求解器选择处报错（而不是按 switch 顺序静默挑稀疏）。
func TestEngineConfigChooseLUSolverHonorsBinding(t *testing.T) {
	con := &element.Context{}
	con.MnaUpdateType = mna.NewMnaUpdateT[float64](1, 0)

	con.EngineCfg.SparseLU, con.EngineCfg.ForceDense = false, false
	lu, sparse, err := chooseLUSolver(con, 1)
	if err != nil || lu == nil {
		t.Fatalf("自动模式应成功: lu=%v err=%v", lu, err)
	}
	if !sparse {
		t.Fatal("零矩阵密度 0 < 0.3 应选稀疏 LU")
	}

	con.EngineCfg.SparseLU = true
	if _, sparse, err = chooseLUSolver(con, 1); err != nil || !sparse {
		t.Fatalf("显式稀疏应成功且 sparse=true: sparse=%v err=%v", sparse, err)
	}

	con.EngineCfg.SparseLU, con.EngineCfg.ForceDense = false, true
	if _, sparse, err = chooseLUSolver(con, 1); err != nil || sparse {
		t.Fatalf("显式稠密应成功且 sparse=false: sparse=%v err=%v", sparse, err)
	}

	con.EngineCfg.SparseLU, con.EngineCfg.ForceDense = true, true
	if _, _, err = chooseLUSolver(con, 1); err == nil {
		t.Fatal("互斥绑定在 chooseLUSolver 必须报错")
	}
}

// TestEngineConfigStrategyBindingMapping 策略绑定字段映射（EngineCfg 仍是唯一数据源）。
func TestEngineConfigStrategyBindingMapping(t *testing.T) {
	con := &element.Context{}
	con.EngineCfg.GminCont = true
	con.EngineCfg.NaturalGmin = 1e-9
	con.EngineCfg.GminFinal = 1e-5
	con.EngineCfg.GlobalGmin = 1e-7
	con.EngineCfg.GminDbg = true
	con.EngineCfg.NoEq = true
	// 后端字段不得混入策略绑定（绑定时刻不同的开关不能互相顶替）。
	con.EngineCfg.SparseLU = true
	con.EngineCfg.ForceDense = false

	s := StrategyBindingOf(con)
	want := StrategyBinding{
		GminCont:    true,
		NaturalGmin: 1e-9,
		GminFinal:   1e-5,
		GlobalGmin:  1e-7,
		GminDbg:     true,
		NoEq:        true,
	}
	if s != want {
		t.Fatalf("策略绑定映射错误：want %+v got %+v", want, s)
	}
	// GminFinal<=0 的折算属 ApplyStrategy（m5）语义，映射层必须原样透传，不得在此折算。
	con.EngineCfg.GminFinal = 0
	if got := StrategyBindingOf(con).GminFinal; got != 0 {
		t.Fatalf("映射层不得折算 GminFinal（<=0 由 ApplyStrategy 归一）：实际 %v", got)
	}
}
