// 引擎配置对象（契约 K3）：后端绑定（载入期、不可变）与策略绑定（运行期、每次仿真实读）。
//
// 本文件把原先散落在 element.Context.EngineCfg（8 个开关）上的读取点收敛成两个对象，
// 并为每一个开关写明**绑定时刻**——这是本轮唯一的新语义，不改任何算法或数值行为：
//
//   - BackendBinding（SparseLU / ForceDense）：矩阵存储与 LU 求解后端在**载入期**
//     （load.LoadContext → element.BackendBinding.Bind）一次性绑定；运行期改写必须报错
//     （文案含"载入期绑定"），禁止部分生效。
//   - StrategyBinding（GminCont / NaturalGmin / GminFinal / GlobalGmin / GminDbg / NoEq）：
//     每次仿真启动时从 Context.EngineCfg 重新读取（StrategyBindingOf），
//     由 m5 在 element/time/strategy.go 的 ApplyStrategy 写入 TimeMNA。
//
// 挂载层（D11 裁决）：BackendBinding 的**定义**在 element/context.go（因为
// element/time 的内部测试依赖 load，load 不能再依赖 element/time，否则测试期形成环）；
// 本文件用类型别名转发，契约里的 `time.BackendBinding` 与其 Validate / ResolveBackend 仍可用。
//
// 过渡期并存：element.Context.EngineCfg 仍是唯一数据源（本文件只做视图与校验），
// 迁移路径见 runs/circuit-perf-optimization/_results/m6/summary.md。
package time

import "circuit/element"

// BackendBinding 引擎后端绑定（契约 K3）：稀疏/稠密 LU 的存储与求解后端选择。
// 定义在 element 包（见 element/context.go 的 D11 说明），此处仅做别名转发：
//
//	type BackendBinding = element.BackendBinding
//
// 载入期绑定由 load.LoadContext 调用 element.BackendBinding.Bind；
// 校验（Validate，运行期改写报错，文案含"载入期绑定"）与实际生效后端解析
// （ResolveBackend）都是该类型的方法。
type BackendBinding = element.BackendBinding

// BackendBindingOf 从 Context 读回后端绑定视图（当前值 + 载入期冻结记录）。
// 运行期写入路径（如 MCP circuit_set_engine_config）应先试写再 Validate：
// 校验失败即返回错误且**不动** Context，禁止部分生效。
func BackendBindingOf(con *element.Context) BackendBinding {
	return con.CurrentBackend()
}

// ---------------- 策略绑定（运行期，每次仿真实读） ----------------

// StrategyBinding 收敛与步长策略绑定（契约 K3）：每次仿真启动时重新读取并生效。
// 与 BackendBinding 的区别是绑定时刻：这些开关在运行期可改，改后下一次仿真生效。
type StrategyBinding struct {
	GminCont    bool    // CIRCUIT_GMCONT：Gmin 延续步进
	NaturalGmin float64 // CIRCUIT_NATGMIN：自然 gmin（对角叠加）
	GminFinal   float64 // CIRCUIT_GMINFINAL：延续终值（<=0 语义由 ApplyStrategy 归一为默认值）
	GlobalGmin  float64 // CIRCUIT_GLOBALGMIN：全局对角 gmin（0=关闭）
	GminDbg     bool    // CIRCUIT_GMIN_DBG：打印 gmin 步进轨迹
	NoEq        bool    // CIRCUIT_NOEQ：跳过行/列均衡化
}

// StrategyBindingOf 从 Context.EngineCfg 读出策略绑定（过渡期桥接；
// EngineCfg 仍是唯一数据源，本函数只做字段映射，不做任何默认值折算）。
func StrategyBindingOf(con *element.Context) StrategyBinding {
	return StrategyBinding{
		GminCont:    con.EngineCfg.GminCont,
		NaturalGmin: con.EngineCfg.NaturalGmin,
		GminFinal:   con.EngineCfg.GminFinal,
		GlobalGmin:  con.EngineCfg.GlobalGmin,
		GminDbg:     con.EngineCfg.GminDbg,
		NoEq:        con.EngineCfg.NoEq,
	}
}

// ---------------- 交接：ApplyStrategy（由 m5 实现，签名与语义在此冻结） ----------------
//
// 契约 K3 冻结的签名（**由 m5 在 element/time/strategy.go 声明并实现**，
// 本文件不声明该符号，避免与 m5 重复定义；D10 裁决已确认）：
//
//	func ApplyStrategy(tm *TimeMNA, s StrategyBinding) error
//
// 语义（冻结原文）：
//   - 每次仿真启动调用一次，**无条件写** TimeMNA（不是"仅当非零才写"）；
//   - s.GminFinal <= 0 的语义 = 回默认值（不是"不修改"）；
//   - 遇 NaN 或负值 → 报错且**不改动** TimeMNA 现有状态（禁止部分生效）；
//   - 不引入跨 run 残留：写入的每个字段都必须能在下一次调用时被重新覆盖
//     （历史教训：GminFinal 复制进 TimeMNA 且不回退会让结果依赖运行历史）。
//
// 交接记录：BoundedNewtonEvents（契约 K4）同样由 m5 在 element/time/strategy.go 声明；
// m6 不声明这两个符号（40-plan.md K4 与 D10 裁决）。
//
// 消费方：m5 在 simulation.go 的瞬态入口调用 ApplyStrategy(tm, StrategyBindingOf(con))。
