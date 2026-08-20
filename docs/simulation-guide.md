# 仿真使用指南

## 运行仿真

### 命令行运行

```bash
# 从项目根目录:CLI 仿真主程序(circuit 子命令)
go run ./cmd -t 0.01 cmd/circuits/02_rc_filter.net

# 输出格式: csv(默认) / tsv / table / html
go run ./cmd -t 0.05 --format html -o result.html cmd/circuits/07_rlc_circuit.net

# 交互式 TUI 连续仿真(实时监控/事件注入/曲线绘制)
go run ./cmd interactive cmd/circuits/10_latch_self_hold.net

# 潮流网表(B/BR)自动识别,直接求解,无需 MCP
go run ./cmd --format table cmd/circuits/29_powerflow_3bus.net

# 运行 MCP 服务器(circuit mcpserver 子命令)
go run ./cmd mcpserver

# 测试电路网表在 cmd/circuits/ 目录
```

CLI 完整选项见 `cmd/circuits/README.md`;潮流 B/BR 网表语法与输出见 [powerflow.md](powerflow.md) 第 3 节。

### 运行测试

```bash
# 所有测试
go test ./...

# 基础元件测试
go test -v ./element/passive/ ./element/semiconductor/ ./element/logic/

# 稳态分析测试(AC 相量/小信号/潮流)
go test -v ./analysis/... ./analysis/powerflow/
```

## 编写仿真程序

### 最小示例

```go
package main

import (
    "fmt"
    _ "circuit/element/register" // 注册全部元件类型(必须)
    "circuit/element/time"
    "circuit/load"
)

func main() {
    // 1. 加载网表(注意:地节点是 -1,不是 0)
    con, err := load.LoadString(`
V1 [1,-1] [0,0,0,0,5]   # DC 5V 电源
R1 [1,2] [1000]          # 1kΩ 电阻
R2 [2,-1] [2000]         # 2kΩ 电阻 → 分压器
`)
    if err != nil {
        panic(err)
    }

    // 2. 创建仿真时间
    con.Time, _ = time.NewTimeMNA(0.01) // 仿真 10ms

    // 3. 获取节点索引（通过节点重编号映射）
    cOut := con.CompactNodeID[2]

    // 4. 运行瞬态仿真
    err = time.TransientSimulation(con, func(v []float64) {
        t := con.CurrentTime()
        fmt.Printf("t=%fs, Vout=%fV\n", t, v[cOut])
    })
    if err != nil {
        fmt.Printf("仿真失败: %s\n", err)
    }
}
```

### 使用 `CompactNodeID` 映射

加载器自动将网表中的节点 ID 重编号为紧凑索引。在回调函数中通过 `CompactNodeID` 查询：

```go
cNode := con.CompactNodeID[原始节点ID]  // 返回 int
voltage := v[cNode]                     // 获取该节点电压
```

### API 参考

| 函数/方法 | 说明 |
|-----------|------|
| `load.LoadString(s string)` | 从字符串加载网表 |
| `load.LoadContext(r io.Reader)` | 从 Reader 加载网表 |
| `time.NewTimeMNA(endTime float64)` | 创建仿真时间控制器 |
| `time.TransientSimulation(con, callback)` | 运行单线程瞬态仿真 |
| `con.ParallelOpts = &element.ParallelOptions{...}` | 配置并行 DoStep(StampWorkers/CacheThreshold) |
| `con.CurrentTime()` | 获取当前仿真时间 |
| `con.Time.GoodIterations()` | 获取已成功完成的时间步数 |
| `config.IsFlag(flag Flag)` | 检查元件 Flags 特性位 |

### Time 接口配置

`mna.Time` 接口提供了完整的仿真配置能力，在创建 `TimeMNA` 后可通过接口方法调整：

```go
con.Time, _ = time.NewTimeMNA(0.01) // 目标时间 10ms

// 配置误差容差
con.Time.SetTolerances(1e-6, 1e-4)   // absTol, relTol

// 配置步长范围
con.Time.SetStepLimits(1e-12, 1e-3)  // minStep, maxStep

// 配置迭代次数限制
con.Time.SetIterationLimits(200, 100) // maxNonlinIter, maxElemIter

// 配置最大时间步数（防死循环）
con.Time.SetMaxTimeSteps(10000)
```

### ConfigFace 接口

元件配置通过 `ConfigFace` 接口统一访问，关键方法：

| 方法 | 说明 |
|------|------|
| `GetName() string` | 元件名称 |
| `PinNum() int` | 外部引脚数 |
| `ValueNum() int` | 参数数量 |
| `VoltageNum() int` | 电压源数量 |
| `IsFlag(flag Flag) bool` | 检查是否设置指定特性位 |
| `Reset(base NodeFace)` | 重置到初始状态 |

特性位检测示例：

```go
if elem.Config().IsFlag(FlagReactive) {
    // 储能元件处理
}
if elem.Config().IsFlag(FlagNonlinear) {
    // 非线性元件处理
}
if elem.Config().IsFlag(FlagCacheStamp) {
    // 可启用盖章缓存优化
}
```

## RTL 半加器仿真

### 电路结构

RTL 半加器由 8 个 NOR 门构成，每个 NOR 门 = 2 个 NPN 三极管 + 3 个电阻：

```
NOR1: A NOR B         → U  = ¬(A ∨ B)
NOR2: A NOR U         → P  = A ∧ ¬B
NOR3: B NOR U         → Q  = ¬A ∧ B
NOR4: P NOR Q         → XNOR = ¬(A ⊕ B)
NOR5: XNOR NOR XNOR   → Sum = A ⊕ B
NOR6: A NOR A         → ¬A
NOR7: B NOR B         → ¬B
NOR8: ¬A NOR ¬B       → Cout = A ∧ B
```

### 输入信号配置

```go
# 脉冲波 (WfPULSE), bias=0, V_max=5, duty=0.5
V2 [10,-1] [5,0,20,0,5,0.5]   # 20Hz → 每 50ms 切换一次
V3 [11,-1] [5,0,10,0,5,0.5]   # 10Hz → 每 100ms 切换一次
```

两路输入在 120ms 内遍历所有 4 种组合：

| 时间 (ms) | A | B | Sum | Cout |
|-----------|---|---|-----|------|
| 0–20 | 1 | 1 | 0 | 1 |
| 25–45 | 0 | 1 | 1 | 0 |
| 50–70 | 1 | 0 | 1 | 0 |
| 75–95 | 0 | 0 | 0 | 0 |
| 100–120 | 1 | 1 | 0 | 1 |

### 半加器网表（完整）

```
# 电源
V1 [100,-1] [0,0,0,0,5]

# 输入信号
V2 [10,-1] [5,0,20,0,5,0.5]
V3 [11,-1] [5,0,10,0,5,0.5]

# NOR1: A NOR B
R1 [100,20] [1000]
R2 [10,21] [10000]
R3 [11,22] [10000]
Q1 [21,20,-1] [false,100]
Q2 [22,20,-1] [false,100]

# NOR2: A NOR U
R4 [100,30] [1000]
R5 [10,31] [10000]
R6 [20,32] [10000]
Q3 [31,30,-1] [false,100]
Q4 [32,30,-1] [false,100]

# NOR3: B NOR U
R7 [100,40] [1000]
R8 [11,41] [10000]
R9 [20,42] [10000]
Q5 [41,40,-1] [false,100]
Q6 [42,40,-1] [false,100]

# NOR4: P NOR Q
R10 [100,50] [1000]
R11 [30,51] [10000]
R12 [40,52] [10000]
Q7 [51,50,-1] [false,100]
Q8 [52,50,-1] [false,100]

# NOR5: XNOR NOR XNOR (Sum)
R13 [100,60] [1000]
R14 [50,61] [10000]
R15 [50,62] [10000]
Q9 [61,60,-1] [false,100]
Q10 [62,60,-1] [false,100]

# NOR6: A NOR A
R16 [100,70] [1000]
R17 [10,71] [10000]
R18 [10,72] [10000]
Q11 [71,70,-1] [false,100]
Q12 [72,70,-1] [false,100]

# NOR7: B NOR B
R19 [100,80] [1000]
R20 [11,81] [10000]
R21 [11,82] [10000]
Q13 [81,80,-1] [false,100]
Q14 [82,80,-1] [false,100]

# NOR8: ¬A NOR ¬B (Cout)
R22 [100,90] [1000]
R23 [70,91] [10000]
R24 [80,92] [10000]
Q15 [91,90,-1] [false,100]
Q16 [92,90,-1] [false,100]
```

## 稳态分析（AC 相量 / 小信号 / 潮流）

时域仿真之外，项目提供三层稳态分析（`analysis` 包 + `analysis/powerflow` 包，
独立于 element 时域体系，详细说明见 [powerflow.md](powerflow.md)）：

```go
import (
    "circuit/analysis"
    "circuit/analysis/powerflow"
)

// L1 线性 AC 相量分析(复数 MNA 一次求解)
res, _ := analysis.AnalyzeAC(`
V1 [1,-1] [1,0,0,0,1]   # AC 1V∠0°
R1 [1,2] [1000]
C1 [2,-1] [100e-9]
`, 1591.55) // 截止频率,|H|=0.707,-45°
// res.NodeVoltages[2] → 相量;res.SourcesPower → 源功率

// 频率扫描
sweep, _ := analysis.SweepAC(netlist, 100, 10000, 200, true)

// L2 小信号混合分析(DC 工作点 + 二极管线性化 + 逻辑门短路)
ss, _ := analysis.AnalyzeSmallSignal(netlist, 1000)
// ss.OperatingPoint[rawID] → 直流偏置;ss.NodeVoltages[rawID] → 小信号响应

// L3 电力系统潮流(标幺值,牛顿-拉夫逊)
pf, _ := powerflow.Solve(powerflow.Input{
    Buses: []powerflow.Bus{
        {ID: 1, Type: powerflow.BusSlack, V: 1.0},
        {ID: 2, Type: powerflow.BusPQ, P: 0.5, Q: 0.2},
    },
    Branches: []powerflow.Branch{{From: 1, To: 2, R: 0, X: 0.1}},
}, nil)
// pf.BusV / pf.GenPower / pf.BranchFlow / pf.TotalLoss
```

MCP 层对应工具：`circuit_run_ac` / `circuit_sweep_ac` / `circuit_run_powerflow`。

CLI 层：潮流网表（B/BR 语法，如 `cmd/circuits/29_powerflow_3bus.net`）由 CLI 自动识别并直接求解，
输出 csv/tsv/table/html 四种格式；AC/小信号分析暂仅经 MCP 或 Go API 运行。

## 已知限制

1. **GUI 已移除**：2026-4 移除 GUI 实现（原 `cmd/app/`），当前交互界面为 TUI（`interactive` 模式）与 MCP 服务器
2. **部分 SPICE 命令不支持**：网表 `.param` 等 SPICE 命令尚未支持
3. **稳态分析限制**：AC 不支持子电路与非线性元件（小信号除外）；小信号工作点不支持 C/L；潮流不支持子电路
