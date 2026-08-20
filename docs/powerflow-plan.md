# 潮流计算与逻辑电路联动 — 深度规划

> 状态:**✅ 已全部实现**(2026-08,提交 7d95dbc~db0e65b,全量测试零回归)
> 使用文档见 [docs/powerflow.md](powerflow.md);本文件保留为规划归档。
> 落地对照:L1 AC 相量 → `analysis/ac.go`;L2 小信号 → `analysis/small_signal.go`;
> L3 潮流 → `analysis/powerflow/`;MCP 3 工具 → `mcp/tools_ac.go`(工具总数 25)。
> 背景:README 承诺「通过事件同步实现 逻辑电路,潮流计算 的联动仿真」;逻辑电路已完整落地,潮流计算目前只有复数化地基。

---

## 1. 目标

把 README 承诺的「潮流计算」真正落地,并打通与逻辑电路(事件系统)的联动,形成三层能力:

| 层 | 能力 | 语义 |
|---|---|---|
| L1 | **AC 相量分析** | 线性电路频域稳态:复数 MNA 一次求解,输出幅值/相位/复功率,支持频率扫描 |
| L2 | **小信号混合分析** | DC 工作点 + 工作点线性化 + AC 小信号,逻辑门/二极管/三极管与 RLC 混仿的频域响应(SPICE `.OP`+`.AC` 最小闭环) |
| L3 | **电力系统潮流** | Slack/PV/PQ 母线模型 + Ybus + 牛顿-拉夫逊迭代,输出母线电压/注入功率/线路潮流/网损 |

三条路径共用同一复数线性代数地基(`maths` 泛型 + `mna.MnaType[T]`),增量实现,互不阻塞。

---

## 2. 现状-差距矩阵(基于代码事实)

### 已就绪(无需改动)

| 组件 | 位置 | 证据 |
|---|---|---|
| 复数线性代数 | `maths/face.go` | `Number` 含 `complex64/128`;`Abs` 复数模(Hypot) |
| 复数 LU 分解 | `maths/lu.go` | 泛型 `LU[T]`,主元选择用 `Abs()`(复数可用);`lu_test.go` 有 complex128 用例 |
| 复数均衡化 | `maths/equilibrate.go` | `scaleNum` 复数分支;`EquilibrateAndDecompose/SolveEquilibrated` 泛型 |
| 复数 MNA 盖章 | `mna/mna.go` | `StampImpedance` 复数 `1/z` 分支;矩阵/右端项复数 NaN 过滤;电压源 `one=1.0+0i` |
| 复数解析 | `load/ast/value.go` | `ParseComplex128`;`StringToAny` 支持 complex128 |
| 事件系统(联动) | `element/context.go` | `EventSlots` + `SetEvent/PushEvents/PullEvents`;B 开关/RLY 线圈/VR 电阻已用 |

### 缺口(需要新建)

| 缺口 | 位置 | 说明 |
|---|---|---|
| 复数 MNA 实例化 | `mna/mna.go` | `NewMna/NewMnaUpdate` 硬编码 `float64`,全库无 `MnaType[complex128]` 实例 |
| 相量电压/电流源 | `element/source/Voltage.go` | `WfAC` 是时域正弦 `v(t)=Vm·sin(ωt+φ)`,无相量模式 |
| AC 分析入口 | 无 | 无频域求解器、无频率扫描 |
| DC 工作点→小信号线性化 | 无 | 非线性元件(逻辑门/D/Q)在偏置点的交流导纳 |
| 母线模型 + Ybus | 无 | 无 Slack/PV/PQ 元件、无 Ybus 构建器 |
| 牛顿-拉夫逊潮流主循环 | 无 | 无雅可比 H/N/J/L、无收敛控制 |
| MCP 工具 | `mcp/server.go` | 22 工具均为时域,无 AC/潮流工具 |

### 关键约束

- `load/load.go:456` 明确拒绝复数参数 → **P0/P1 的 AC 分析不侵入 element 体系**(自建轻量解析),规避类型传播风险
- `element/time` TimeMNA 全 `float64` → 潮流/AC 是**稳态分析,不需要 TimeMNA**(一次求解或独立迭代),零冲突
- MCP 注册模式已清晰(`register(tool, handler)` + `tools_*.go` 处理器),只增不改,无回归

---

## 3. 总体架构

```
                    ┌──────────────────────────────────────┐
                    │            L3 powerflow 包           │
                    │  母线模型 + Ybus + 牛顿-拉夫逊        │
                    └───────────────┬──────────────────────┘
                                    │ 依赖复数 LA + MNA
┌───────────────┐   ┌───────────────▼──────────────────────┐
│ element 体系   │   │            L1/L2 analysis 包          │
│ (时域+逻辑+事件)│──▶│  AC 相量求解 / DC工作点 + 小信号 AC    │
│  已落地不动    │   └───────────────┬──────────────────────┘
└───────────────┘                   │
                    ┌───────────────▼──────────────────────┐
                    │   mna.MnaType[complex128] 实例化       │
                    │   (NewMnaComplex 泛型工厂,新文件)      │
                    └───────────────┬──────────────────────┘
                    ┌───────────────▼──────────────────────┐
                    │  maths 泛型复数 LA (已有,零改动)       │
                    └──────────────────────────────────────┘
```

- **L1/L2(`analysis` 包)独立于 element 体系**:只依赖 `load/ast`(语法解析)+ `maths` + `mna`。同网表、两种分析——AC 分析直接复用现有 Voltage 网表格式(`waveform=1` 即相量,`V_max`=幅值,`phase`=相位),用户无学习成本
- **L3(`powerflow` 包)**:自描述输入结构体(母线+支路),不走 SPICE 网表;可后续再加网表方言
- **联动**:L2 用现有 `TransientSimulation` 求 DC 工作点(纯 DC 电路自适应大步长秒收敛,或 MCP `run_dc`),逻辑门输出在工作点处视为理想电压源(小信号短路)——逻辑电路与网络求解的耦合已有事件系统兜底,本规划**不改动** element/context.go

---

## 4. 阶段 1(P0):线性 AC 相量分析 — 2~3 天

### 4.1 新增文件

| 文件 | 内容 |
|---|---|
| `mna/complex.go` | `NewMnaComplex(nodes, vs) MnaComplex` + 泛型工厂 `NewMnaUpdateT[T]`(约 40 行) |
| `analysis/ac.go` | AC 求解器:`ACResult`、`AnalyzeAC`、`SweepAC`(约 300 行) |
| `analysis/parse.go` | 轻量网表解释:R/C/L/V/I 相量参数提取(复用 `load/ast`,约 150 行) |
| `analysis/ac_test.go` | 见 4.4(约 200 行) |

### 4.2 核心接口

```go
package analysis

// ACResult 单频相量分析结果
type ACResult struct {
    Frequency     float64
    NodeVoltages  map[int]complex128 // 节点ID → 相量电压 Vm∠φ
    BranchCurrent map[string]complex128 // 元件实例名 → 相量电流
    NodePowers    map[int]complex128 // 节点复功率 S = V · conj(I) (注入聚合)
    TotalPower    complex128        // 全网复功率(校验守恒)
}

// AnalyzeAC 线性 AC 相量分析(一次复数 MNA 求解)
// netlist 复用现有语法:V1 [1,0] [1, bias, freq, phase, V_max]
//   waveform=1(AC) → 相量 V_max∠(phase°);频率默认取参数 freq,可由 frequency 覆盖
func AnalyzeAC(netlist string, freq float64) (*ACResult, error)

// SweepAC 频率扫描(对数/线性),返回幅频/相频曲线
func SweepAC(netlist string, fMin, fMax float64, points int, logScale bool) (*SweepResult, error)

type SweepResult struct {
    Frequencies []float64
    Magnitudes  map[int][]float64 // 节点ID → 幅值序列
    Phases      map[int][]float64 // 节点ID → 相位序列(度)
}
```

### 4.3 算法要点

1. **Ybus 构建**(复数):R→`1/R`;L→`1/(jωL)`;C→`jωC`,`StampAdmittance` 盖章
2. **相量源**:V→`StampVoltageSource(n1,n2,id, Vm·e^{jφ})`;I→`StampCurrentSource` 注入 `Im·e^{jφ}`
3. **一次求解**:`maths.NewLU[complex128]` + `EquilibrateAndDecompose`(复数缩放已就绪)→ `SolveEquilibrated`
4. **后处理**:`|V|`,`∠V`(atan2)、支路电流、`S_k = V_k·conj(I_k)`
5. 数值卫生:解中 NaN/Inf 检测(复用 `IsValidComplex128`)

### 4.4 测试用例(理论可验证)

| 电路 | 理论值 |
|---|---|
| RC 低通(1kΩ, 100nF) | `|H|=1/√(1+(ωRC)²)`,`∠H=-arctan(ωRC)`;f=1.59kHz 处 -3dB、-45° |
| RLC 串联谐振 | 谐振 f₀=1/(2π√(LC)),Q 因子、带宽 |
| 电阻分压 + 相位 | 纯阻网络 ∠=0,分压比精确 |
| 频率扫描 | 幅频曲线与理论 Bode 逐点一致(容差 1e-6) |

### 4.5 验收标准

- RC 电路 -3dB 频率与理论误差 < 0.1%;相位曲线全频段误差 < 0.1°
- 复功率守恒:`ΣS_generator ≈ ΣS_load + ΣS_loss`(1e-9 标幺)
- `go test ./analysis/...` 全绿;现有全部测试零回归

---

## 5. 阶段 2(P1):DC 工作点 + 小信号 AC(逻辑联动)— 2 天

### 5.1 新增文件

| 文件 | 内容 |
|---|---|
| `analysis/small_signal.go` | 工作点求解 + 小信号线性化 + AC(约 250 行) |
| `analysis/linearize.go` | 非线性元件工作点导纳:D 二极管、Q 三极管、逻辑门、MOSFET(约 200 行) |
| `analysis/small_signal_test.go` | 混合电路测试(约 150 行) |

### 5.2 算法要点(SPICE .OP + .AC 最小闭环)

1. **DC 工作点**:现有 `element/time.TransientSimulation`(纯 DC 电路大步长秒收敛,或复用 MCP `run_dc` 路径)→ 得全节点 `V_dc`
2. **工作点线性化**(每个元件 → 小信号复数导纳):
   - R/C/L:直接 `1/R`、`jωC`、`1/(jωL)`
   - 二极管:`g_d = I_s/V_T · e^{V_d/V_T}`(工作点切线)
   - 三极管/MOSFET:工作点跨导 `g_m`、输出电导 `g_o`(Shichman-Hodges 求导)
   - **逻辑门(联动点)**:输出在工作点为 0 或 V_high → 视为**理想电压源(小信号短路)**;输入阻抗视为无穷
   - 独立电压源:小信号短路;独立电流源:小信号开路
3. **小信号 AC**:构建复数 MNA,在 DC 源处注入单位小信号相量(源 1∠0°),一次求解 → 幅值/相位(即传递函数 H(jω))

### 5.3 测试用例

| 电路 | 理论值 |
|---|---|
| 逻辑门(5V)驱动 RC 网络 | 小信号增益 0dB(理想电压源驱动),转折频率同 P0 RC 算例 |
| 二极管偏置点 g_d 验证 | g_d 与理论 `I/V_T` 一致(1%) |
| CE 共射放大级 | 中频增益 ≈ -g_m·Rc(10% 容差,简化模型) |
| 反相门→同相放大混合 | 逻辑高电平下运放闭环增益与设计值一致 |

### 5.4 验收标准

- 纯电阻网络的小信号结果与 P0 线性 AC 完全一致(交叉验证)
- 逻辑门驱动电路在直流电平处的 AC 响应与「理想源 + RLC」手算一致
- 无 element 体系改动(事件系统零触碰,回归为零)

---

## 6. 阶段 3(P2):电力系统潮流(牛顿-拉夫逊)— 3~4 天

### 6.1 新增文件

| 文件 | 内容 |
|---|---|
| `analysis/powerflow/bus.go` | 母线/支路/输入输出结构体(约 120 行) |
| `analysis/powerflow/ybus.go` | Ybus 构建(含变压器变比、对地导纳可选)(约 120 行) |
| `analysis/powerflow/newton.go` | 极坐标牛顿-拉夫逊主循环 + 雅可比(约 300 行) |
| `analysis/powerflow/powerflow_test.go` | 算例验证(约 250 行) |

### 6.2 核心接口

```go
package powerflow

type BusType int
const (BusSlack BusType = iota; BusPV; BusPQ)

type Bus struct {
    ID     int
    Type   BusType
    V      float64 // 幅值标幺(Slack/PV 给定;PQ 初值 1.0)
    Theta  float64 // 相角(弧度,Slack 给定,其余初值 0)
    P, Q   float64 // 注入(Slack 不给定;PV 给 P;PQ 给 P,Q)
    Qmin, Qmax float64 // PV 无功限幅(可选,越限转 PQ)
}

type Branch struct {
    From, To int      // 母线 ID
    R, X     float64  // 串联阻抗(标幺)
    B        float64  // 对地导纳(可选,默认 0)
    Tap      float64  // 变比(可选,默认 1)
}

type Input struct {
    Frequency float64
    BaseMVA   float64 // 标幺基值(默认 100)
    Buses     []Bus
    Branches  []Branch
}

type Result struct {
    Converged  bool
    Iterations int
    BusV       map[int]complex128   // 母线电压相量
    GenPower   map[int]complex128   // Slack/PV 注入 S=P+jQ
    BranchFlow map[int]BranchFlow   // 支路始/末端 P,Q、损耗
    TotalLoss  complex128
}

func Solve(in Input, opts *Options) (*Result, error)
// Options: Tol(默认1e-6)、MaxIter(默认50)、FlatStart(默认true)、Damping(默认1.0)
```

### 6.3 算法要点

1. **Ybus**:`Y[i][i]=Σy_ik+y_i0`,`Y[i][k]=-y_ik`(含变比 `y/t`、`y/t²` 修正)
2. **极坐标牛顿-拉夫逊**:未知量 = PQ 母线 θ,V + PV 母线 θ;方程 = PQ 的 ΔP/ΔQ + PV 的 ΔP
3. **雅可比四子块**:H=∂P/∂θ、N=∂P/∂V、J=∂Q/∂θ、L=∂Q/∂V(标准公式),用 `maths.NewLU[float64]` 求解增量 `[Δθ; ΔV/V]`
4. **收敛控制**:`max|ΔP|, max|ΔQ| < Tol`;flat start(V=1.0∠0°)初值;发散时阻尼因子回退
5. **PV 无功越限**:Q 超 `[Qmin,Qmax]` → 转 PQ(固定 Q 边界),经典处理
6. **后处理**:线路潮流 `S_ik = V_i·conj(I_ik)`,网损 `Σ(S_gen) - Σ(S_load)`

### 6.4 测试用例(可解析验证)

| 算例 | 验证方式 |
|---|---|
| 2 母线:Slack + PQ(线路 Z=j0.1) | 手推解析解(两方程两未知)逐位比对 |
| 3 母线经典算例(教科书) | 已知解比对(1e-6) |
| PV 无功越限 | 触发转 PQ,结果与手算一致 |
| 病态网络(高 R/X) | 阻尼回退路径触发,不崩溃 |

### 6.5 验收标准

- 2 母线解析解误差 < 1e-6;3 母线教科书算例一致
- 功率平衡:`ΣP_gen = ΣP_load + ΣP_loss`(1e-9)
- 越限/发散路径有明确错误返回,不 panic

---

## 7. 阶段 4(P3):MCP 工具暴露 — 2 天

### 7.1 新增工具(只增不改,零回归)

| 工具 | 参数 | 返回 |
|---|---|---|
| `circuit_run_ac` | sessionId, frequency, nodes? | 各节点 幅值/相位/复功率(JSON) |
| `circuit_sweep_ac` | sessionId, fMin, fMax, points | 幅频/相频数据(供 export_plot 复用 HTML 波形页) |
| `circuit_run_powerflow` | netlist(powerflow 方言)或结构化参数 | 母线表(类型/V/θ/P/Q)、线路潮流、网损、迭代次数 |

### 7.2 实现要点

- `mcp/tools_simulate.go` 增加注册;处理器在 `mcp/handler_ac.go`(新文件)
- 会话侧:AC/潮流是**稳态瞬算**(秒级),走同步路径,不占 jobs 异步框架;大规模潮流可后续接 `jobs.go`
- 结果序列化:复数一律输出 `{"mag":..,"phase_deg":..}` 结构,不裸序列化 complex128
- `export_plot` 复用:AC 幅频曲线以「频率为横轴」的曲线数据输入,复用现有自包含 HTML 波形页

### 7.3 验收

- `integration_test.go` 增加 AC + 潮流用例;22 个既有工具测试全绿
- `docs/mcp.md` 工具清单更新至 25 个

---

## 8. 阶段 5:P4 文档与验证电路 — 1 天

| 交付 | 内容 |
|---|---|
| `docs/powerflow.md` | 三层能力的使用指南:网表语法、参数、示例、理论对照 |
| `cmd/circuits/` | 新增 12_ac_filter.net、13_small_signal.net、14_powerflow_3bus.net |
| `README.md` | 更新能力清单与开发日志 |
| `PROJECT_MEMORY.md` | 沉淀复数化边界、避坑(见 §10) |

---

## 9. 里程碑与依赖

```
M1 (P0) AC 相量分析  ── 2~3 天
   │ 依赖: maths/mna 复数层(已就绪)
M2 (P1) 小信号混合    ── 2 天
   │ 依赖: M1 + element/time(现有)
M3 (P2) 潮流计算      ── 3~4 天
   │ 依赖: maths float64 LU + 均衡化(已就绪),与 M1/M2 并行
M4 (P3) MCP 工具      ── 2 天(依赖 M1/M3)
M5 (P4) 文档/算例     ── 1 天
合计:约 10~12 天(可并行 M1/M2 与 M3)
```

**并行策略**:M1/M2(analysis 包)与 M3(powerflow 包)文件互不重叠,可双线并行;M4 依赖两者的入口。

---

## 10. 风险与应对

| 风险 | 等级 | 应对 |
|---|---|---|
| 复数类型侵入 element 体系(`load/load.go` 拒绝复数参数) | 中 | P0/P1 在独立 `analysis` 包自解析,不触碰 element;若未来需元件复数化,单列任务放开 `setElementValues` |
| 潮流迭代发散(病态/高 R/X) | 中 | flat start + 阻尼回退 + 最优乘子;明确错误返回不 panic |
| 小信号线性化精度(三极管简化) | 低 | 验收用简化模型容差(10%);文档标注模型假设 |
| 现有 22 工具回归 | 低 | MCP 只增不改;integration_test 全量回归 |
| 复数 LU 数值稳定性(均衡化缩放) | 低 | 已就绪(`scaleNum` 复数分支);P0 测试覆盖跨数量级电路 |
| 频率扫描数据量大 | 低 | SweepResult 抽稀/限制点数上限 |

---

## 11. 决策点(待确认)

1. **范围档位**:P0 先行(推荐,2~3 天见效) / P0+P1(混合分析) / P0+P1+P2(全链路) / 仅 P2(直接电力系统潮流)
2. **网表方言**:AC 复用现有 Voltage 格式(`waveform=1` + phase,推荐)vs 新 `AC` 关键字方言
3. **潮流输入**:结构化参数(推荐)vs SPICE 方言网表
4. **MCP 时序**:同步瞬算(推荐,秒级)vs 接入 jobs 异步框架
