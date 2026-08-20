# AC 相量分析 · 小信号混合分析 · 潮流计算

三层稳态分析能力,全部独立于时域仿真(element 体系零改动),共享复数线性代数地基:

| 层 | 入口 | 语义 | 适用 |
|---|---|---|---|
| L1 | `analysis.AnalyzeAC` / `SweepAC` | 线性 AC 相量分析:复数 MNA 一次求解 | RLC 网络频域响应、滤波器、谐振 |
| L2 | `analysis.AnalyzeSmallSignal` | DC 工作点 + 工作点线性化 + AC 小信号 | 二极管偏置、逻辑门驱动网络的频域响应 |
| L3 | `powerflow.Solve` | 电力系统潮流:牛顿-拉夫逊 | Slack/PV/PQ 母线网络、输电线路 |

MCP 工具:`circuit_run_ac` / `circuit_sweep_ac` / `circuit_run_powerflow`。

---

## 1. AC 相量分析(L1)

网表语法与时域仿真一致(节点 `-1` = 地,节点 `0` 是普通节点):

```
V1 [1,-1] [1,0,0,0,1]     # waveform=1(AC):相量 1V∠0°
R1 [1,2] [1000]
C1 [2,-1] [100e-9]
```

| 元件 | 语法 | AC 导纳/相量 |
|---|---|---|
| R | `R1 [n1,n2] [R]` | 1/R |
| C | `C1 [n1,n2] [C]` | jωC |
| L | `L1 [n1,n2] [L]` | 1/(jωL) |
| V | `V1 [n+,n-] [waveform,bias,frequency,phase,V_max]` | waveform=0:(V_max+bias)∠0°;waveform=1:V_max∠phase° |
| I | `I1 [n+,n-] [I]` | I∠0° |

```go
res, err := analysis.AnalyzeAC(netlist, 1591.55) // 频率 Hz;<=0 用网表 V 源 frequency
// res.NodeVoltages[rawID]  相量电压
// res.BranchCurrent[实例名] 支路电流(方向 n1→n2)
// res.NodePowers[rawID]   节点复功率 S=V·conj(I)
// res.TotalPower ≈ 0(守恒校验);res.SourcesPower(负 = 源净输出)

sweep, err := analysis.SweepAC(netlist, 100, 1e6, 200, true) // 对数扫描
// sweep.Frequencies / sweep.Magnitudes[node] / sweep.Phases[node](度)
```

**注意**:电路必须至少一个引脚接地(`-1`),浮空电路无唯一解会报错。

## 2. 小信号混合分析(L2)

同网表双分析:DC 工作点(element 时域体系收敛,AC 源置 0V)+ 小信号 AC。

```
V1 [1,-1] [0,5,0,0,0]       # DC 源:工作点 5V,小信号短路
R1 [1,2] [1000]             # 偏置电阻
D1 [2,-1] [1e-12,0,1,0,300.15]  # 二极管 [Is,Vz,N,Rs,T]
V2 [3,-1] [1,0,1000,0,0.1]  # AC 源:工作点短路,小信号 0.1V∠0°
R2 [3,2] [100000]           # 信号注入电阻
```

| 元件 | 工作点 | 小信号 |
|---|---|---|
| V(waveform=0) | V_max+bias | 短路(0V 理想源) |
| V(waveform=1) | 0V 短路 | V_max∠phase° |
| I | I | 开路(0) |
| D 二极管 | 完整模型 | 电导 g_d = Is/(N·Vt)·e^(Vd/(N·Vt))(工作点切线) |
| U 逻辑门 | 数字电平输出 | 输出短路(理想电压源)、输入高阻(1e12Ω) |

**限制**:工作点要求纯 DC 电路(含 C/L 储能元件报错);三极管/运放等未线性化(报错)。

```go
res, err := analysis.AnalyzeSmallSignal(netlist, 1000)
// res.OperatingPoint[rawID]  DC 工作点电压
// res.NodeVoltages[rawID]    小信号响应(幅值/相位)
```

## 3. 潮流计算(L3)

标幺值输入;母线编号任意整数;`theta` 用弧度(代码)/度(MCP 工具)。

```go
in := powerflow.Input{
    Buses: []powerflow.Bus{
        {ID: 1, Type: powerflow.BusSlack, V: 1.0},
        {ID: 2, Type: powerflow.BusPV, V: 1.0, P: 0.5, Qmax: 0.3, Qmin: -0.3},
        {ID: 3, Type: powerflow.BusPQ, P: -1.0, Q: -0.5},
    },
    Branches: []powerflow.Branch{
        {From: 1, To: 2, R: 0.01, X: 0.1},
        {From: 2, To: 3, R: 0.01, X: 0.1},
    },
}
res, err := powerflow.Solve(in, nil)
// res.Converged / Iterations / BusV[ID] / GenPower[ID]
// res.BranchFlow["1->2"]{PFrom,QFrom,PTo,QTo} / res.TotalLoss
```

- **Slack**:V、θ 给定,提供功率缺口(全网必须恰好 1 条)
- **PV**:V、P 给定;无功超 [Qmin,Qmax] 自动转 PQ(固定 Q 于边界)
- **PQ**:P、Q 给定,求 V、θ(平启动 V=1.0∠0°)
- 支路:`{from, to, r, x, b?, tap?}`(对地电纳 B、变比 tap 可选)
- 算法:极坐标牛顿-拉夫逊(H/N/J/L 雅可比,ΔV/V 形式)+ 均衡化 LU;收敛判据 max|ΔP|,|ΔQ| < tol(默认 1e-6)
- 验证:2 母线解析解(1e-5)、3 母线功率平衡(1e-6)

### CLI 批处理(B/BR 潮流网表)

CLI 自动识别 B/BR 潮流网表(母线行 `B<id> slack|pv|pq ...`、支路行 `BR<id> From-To ...`,语义与 `circuit_run_powerflow` 一致),无需 MCP 即可命令行求解:

```
go run ./cmd -format table circuits/29_powerflow_3bus.net   # 终端表格
go run ./cmd -format csv   circuits/29_powerflow_3bus.net   # CSV(默认格式)
go run ./cmd -format html -o result.html circuits/29_powerflow_3bus.net  # 自包含 HTML 页
```

B/BR 语法(全部标幺值;`Theta` 单位为度;参数键不区分大小写):

```
# 注释(整行 # 开头)
B1 slack V=1.05                      # 平衡母线:V 必给
B2 pv   V=1.0 P=0.5 Qmin=-0.3 Qmax=0.3  # 发电机母线:V、P 必给,无功限幅可选
B3 pq   P=-1.0 Q=-0.5                # 负荷母线:P、Q 必给
BR1 1-2 R=0.01 X=0.1                 # 支路:R/X 必给且不可同时为 0,B/Tap 可选
```

注意:元件引擎也有类型 `B`(EventSwitch 开关,语法 `B1 [1,0] [...]`),CLI 仅当第二个词元为 slack/pv/pq 时判定为潮流母线,普通开关网表不受影响。

## 4. MCP 工具

```
circuit_run_ac        {sessionId, frequency?, nodes?}
circuit_sweep_ac      {sessionId, fMin, fMax, points?, logScale?, nodes?}
circuit_run_powerflow {buses, branches, baseMva?, frequency?, tol?, maxIter?, damping?, flatStart?}
```

功率/复数一律输出 `{p, q}` / `{mag, phaseDeg, real, imag}` 结构(不裸序列化 complex128)。

## 5. 验证电路

| 文件 | 电路 | 理论锚点 |
|---|---|---|
| `cmd/circuits/27_ac_filter.net` | RC 低通 1kΩ+100nF | f0≈1591.55Hz 处 -3dB、-45° |
| `cmd/circuits/28_small_signal.net` | 二极管偏置 + AC 注入 | Vd≈0.57V,g_d≈0.17S,V_ac≈5.8µV |
| `cmd/circuits/29_powerflow_3bus.net` | 3 母线环网(CLI 自动识别 B/BR 网表,或经 MCP `circuit_run_powerflow` 运行) | 功率平衡、PV 保幅 |

## 6. 已知限制

- 网表子电路(.subckt)暂不支持
- 小信号工作点不支持 C/L;三极管/运放未线性化
- 潮流为稠密矩阵求解(母线规模 ≤ 数百条)
- I 源无 AC 相量参数(小信号恒开路)
