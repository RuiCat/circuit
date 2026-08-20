# Circuit Test Netlists / 电路测试网表

本目录包含用于测试和演示 `circuit` 电路仿真器的网表示例文件。

This directory contains example netlist files for testing and demonstrating the `circuit` circuit simulator.

---

## 文件列表 / File List

### 时域仿真网表（01–26）

| 文件 | 描述 | Description |
|------|------|-------------|
| `01_resistor_divider.net` | 电阻分压器 — 最简单的纯电阻电路 | Resistor divider — simplest resistive circuit |
| `02_rc_filter.net` | RC 低通滤波器 — 含储能元件，展示瞬态响应 | RC low-pass filter — energy storage element, transient response |
| `03_diode_rectifier.net` | 二极管半波整流器 — 含非线性元件 | Diode half-wave rectifier — nonlinear element |
| `04_transistor_switch.net` | NPN 三极管开关 — 含非线性元件 | NPN transistor switch — nonlinear element |
| `05_opamp_amplifier.net` | 运放同相放大器 — 含非线性元件 | Op-amp non-inverting amplifier — nonlinear element |
| `06_voltage_sources.net` | 多种电压源波形演示 — 7 种波形 | Voltage source waveform demo — 7 waveforms |
| `07_rlc_circuit.net` | RLC 串联振荡电路 — 含 R、L、C | RLC series oscillator — R, L, C |
| `08_subcircuit.net` | 子电路示例 — RC 滤波器级联 | Subcircuit example — cascaded RC filters |
| `09_half_adder.net` | RTL 半加器 — 8-NOR 门数字逻辑 | RTL half adder — 8-NOR gate digital logic |
| `10_latch_self_hold.net` | 自锁启停电路 — 继电器自保持控制 | Self-holding latch circuit — relay self-hold control |
| `11_mixed_domain.net` | 混合物理域 — 电气/气动/液压跨域仿真 | Mixed domain — electrical/pneumatic/hydraulic |
| `12_current_source.net` | 电流源（诺顿源）驱动电阻负载 | Current source (Norton) driving resistive load |
| `13_controlled_sources.net` | 四种受控源：VCVS(E)/VCCS(G)/CCCS(F)/CCVS(H) | Four controlled sources: E/G/F/H |
| `14_zener_regulator.net` | 齐纳稳压管 — 反向击穿稳压 | Zener diode voltage regulator |
| `15_mosfet_switch.net` | NMOS 共源开关 — Shichman-Hodges 模型 | NMOS common-source switch |
| `16_led_driver.net` | LED 驱动 — 串联限流电阻 | LED driver with series current-limit resistor |
| `17_logic_gates.net` | 原生逻辑门 U：AND/NOT/OR/XOR | Native logic gates U: AND/NOT/OR/XOR |
| `18_d_flipflop.net` | D 触发器 DFF — 时钟上升沿采样 | D flip-flop — rising-edge sampling |
| `19_comparator_schmitt.net` | 比较器 CMP + 施密特触发器 ST | Comparator CMP & Schmitt trigger ST |
| `20_switches.net` | 普通开关 sw — state 参数控制通断 | Plain switch — state-controlled |
| `21_dc_motor.net` | 直流电机 motor — 反电动势模型 | DC motor — back-EMF model |
| `22_transformer.net` | 变压器 xfmr — 初级 AC、次级按匝数比 | Transformer — AC primary, ratio scaling |
| `23_pneumatic_full.net` | 气路全套：PS/PR/PC/PL/PCV | Pneumatic: PS/PR/PC/PL/PCV |
| `24_hydraulic_circuit.net` | 液压回路：HP/HR/HA/HL/HCV | Hydraulic: HP/HR/HA/HL/HCV |
| `25_current_sensor.net` | 电流传感器 CS — 支路电流→电压 | Current sensor — branch current to voltage |
| `26_eh_converter.net` | 电气-液压转换器 EH | Electro-hydraulic converter EH |

### 稳态分析网表（27–29，非时域）

| 文件 | 描述 | Description |
|------|------|-------------|
| `27_ac_filter.net` | RC 低通 1kΩ+100nF — AC 相量分析/频率扫描 | AC phasor analysis / sweep (f0≈1591.55Hz, -3dB) |
| `28_small_signal.net` | 二极管偏置 + AC 注入 — 小信号混合分析 | Diode bias + AC injection — small-signal |
| `29_powerflow_3bus.net` | 3 母线潮流环网 — B/BR 网表（CLI 直接运行） | 3-bus power flow — B/BR netlist (CLI-runnable) |

> 27/28 需经 MCP `circuit_run_ac` / `circuit_sweep_ac` / 稳态分析 API 运行（见 [docs/powerflow.md](../../docs/powerflow.md)）；
> 29 由 CLI 自动识别 B/BR 潮流网表直接求解。

---

## 运行示例 / Running Examples

```bash
# 电阻分压器
circuit -t 0.001 cmd/circuits/01_resistor_divider.net

# RC 低通滤波器
circuit -t 0.01 cmd/circuits/02_rc_filter.net

# 二极管整流器
circuit -t 0.04 cmd/circuits/03_diode_rectifier.net

# 三极管开关
circuit -t 0.001 cmd/circuits/04_transistor_switch.net

# 运放放大器
circuit -t 0.001 cmd/circuits/05_opamp_amplifier.net

# 多波形演示 (只输出节点1-5)
circuit -t 0.04 --nodes 1,2,3,4,5 cmd/circuits/06_voltage_sources.net

# RLC 电路
circuit -t 0.002 cmd/circuits/07_rlc_circuit.net

# 子电路级联
circuit -t 0.05 --nodes 1,2,3,4 cmd/circuits/08_subcircuit.net

# 半加器
circuit -t 0.15 --nodes 60,90 cmd/circuits/09_half_adder.net

# 自锁启停电路 (交互式)
circuit interactive cmd/circuits/10_latch_self_hold.net

# 混合物理域 (电气-气动-液压)
circuit -t 0.1 cmd/circuits/11_mixed_domain.net

# 电流源
circuit -t 0.001 cmd/circuits/12_current_source.net

# 受控源
circuit -t 0.001 cmd/circuits/13_controlled_sources.net

# 齐纳稳压
circuit -t 0.01 cmd/circuits/14_zener_regulator.net

# MOSFET 开关
circuit -t 0.001 cmd/circuits/15_mosfet_switch.net

# LED 驱动
circuit -t 0.01 cmd/circuits/16_led_driver.net

# 逻辑门
circuit -t 0.01 cmd/circuits/17_logic_gates.net

# D 触发器
circuit -t 0.02 cmd/circuits/18_d_flipflop.net

# 比较器/施密特触发器
circuit -t 0.01 cmd/circuits/19_comparator_schmitt.net

# 开关
circuit -t 0.001 cmd/circuits/20_switches.net

# 直流电机
circuit -t 0.05 cmd/circuits/21_dc_motor.net

# 变压器
circuit -t 0.05 cmd/circuits/22_transformer.net

# 气路全套
circuit -t 0.1 cmd/circuits/23_pneumatic_full.net

# 液压回路
circuit -t 0.1 cmd/circuits/24_hydraulic_circuit.net

# 电流传感器
circuit -t 0.001 cmd/circuits/25_current_sensor.net

# 电气-液压转换器
circuit -t 0.1 cmd/circuits/26_eh_converter.net

# 潮流计算 (B/BR 网表, CLI 自动识别)
circuit --format table cmd/circuits/29_powerflow_3bus.net
```

---

## 命令行选项 / Command-Line Options

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `-t`, `--time` | 0.1 | 仿真目标时间（秒） |
| `-o`, `--output` | stdout | 输出文件路径 |
| `--step` | 1e-6 | 初始步长 |
| `--min-step` | 1e-9 | 最小步长 |
| `--max-step` | 5e-4 | 最大步长 |
| `--abs-tol` | 1e-6 | 绝对容差 |
| `--rel-tol` | 1e-4 | 相对容差 |
| `--max-iter` | 200 | 最大非线性迭代次数 |
| `--max-steps` | 10000 | 最大时间步数 |
| `--nodes` | (全部) | 逗号分隔的输出节点 ID 列表，如 `1,2,5` |
| `--parallel` | 0 | 并行 worker 数（0=串行模式） |
| `--format` | csv | 输出格式: `csv`, `tsv`, `table`, `html` |
| `--quiet` | false | 静默模式，不输出元信息到 stderr |
| `-h`, `--help` | - | 显示帮助 |

用法: `circuit [选项] <网表文件>`

`--format html` 输出自包含 HTML 波形页（内联 Canvas 交互图表），须配合 `-o <文件.html>` 保存：

```bash
circuit -t 0.05 --format html -o result.html cmd/circuits/07_rlc_circuit.net
```

---

## 网表格式快速参考 / Netlist Format Quick Reference

### 通用语法 / General Syntax
```
<类型><ID> [引脚节点列表] [参数值列表]
<type><ID> [pin nodes] [parameter values]
```

### 关键规则 / Key Rules

- **节点 `-1`** 是全局地节点（电压=0），必须用 -1 接地。
- 节点 ID 为非负整数（如 1,2,10,100），加载器自动重编号。
- **注释**: `#` 或 `//` 行注释；`/* */` 块注释。
- **子电路**: `.subckt <name> <port1> ...` 到 `.ends <name>`，用 `X<ID> [引脚...] [子电路名]` 实例化。
- **变量**: `.value <name> <value>` 定义，`%name` 引用。

- **Node `-1`** is the global ground node (voltage=0). Must use -1 for ground connections.
- Node IDs are non-negative integers (e.g. 1, 2, 10, 100). The loader auto-renumbers nodes.
- **Comments**: `#` or `//` line comments; `/* */` block comments.
- **Subcircuits**: `.subckt <name> <port1> ...` to `.ends <name>`. Instantiate with `X<ID> [pins...] [subcircuit name]`.
- **Variables**: `.value <name> <value>` defines, `%name` references.

### 元件格式 / Component Formats

#### 电气域 / Electrical

| 元件 | 格式 | 示例 |
|------|------|------|
| **R** 电阻 | `R<id> [v+,v-] [阻值Ω]` | `R1 [1,-1] [1000]` |
| **C** 电容 | `C<id> [v+,v-] [容值F]` | `C1 [2,-1] [1e-6]` |
| **L** 电感 | `L<id> [v+,v-] [感值H]` | `L1 [1,2] [1e-3]` |
| **D** 二极管 | `D<id> [anode,cathode] [Is,Vz,N,Rs,T]`（方向由引脚顺序决定：anode→cathode 正向） | `D1 [1,2] [1e-14,0]` |
| **Q** NPN三极管 | `Q<id> [base,collector,emitter] [is_PNP,betβ]` | `Q1 [2,3,-1] [false,100]` |
| **V** 电压源 | `V<id> [v+,v-] [波形,bias,频率,相位,Vmax,duty]` | 见下方 |
| **I** 电流源 | `I<id> [v+,v-] [电流A]` | `I1 [-1,1] [0.01]` |
| **opamp** 运放 | `opamp<id> [noninv+,inv-,out] [V+,V-,开环增益]` | `opamp1 [1,2,3] [15,-15,1e5]` |
| **E** VCVS | `E<id> [cp,cn,op,on] [增益]` | `E1 [1,-1,2,-1] [2]` |
| **G** VCCS | `G<id> [cp,cn,op,on] [gm]` | `G1 [3,-1,4,-1] [0.001]` |
| **F** CCCS | `F<id> [cp,cn,op,on] [增益]` | `F1 [6,-1,8,-1] [2]` |
| **H** CCVS | `H<id> [cp,cn,op,on] [跨阻Ω]` | `H1 [10,-1,12,-1] [1000]` |
| **Z** 齐纳管 | `Z<id> [anode,cathode] [Is,Vz,N,Rs,T]` | `Z1 [-1,2] [1e-14,5.1,1,0.1]` |
| **LED** 发光二极管 | `LED<id> [anode,cathode] [Is,Vz,N,Rs,T]` | `LED1 [2,-1] [1e-18,0,1.5,5]` |
| **M** MOSFET | `M<id> [drain,gate,source] [type,VTO,KP,LAMBDA]`（type 0=NMOS/1=PMOS） | `M1 [2,3,-1] [0,1.5,2e-5,0.01]` |
| **J** JFET | `J<id> [drain,gate,source] [type,VTO,BETA,LAMBDA]` | `J1 [2,3,-1] [0,2,1e-4,0.02]` |
| **sw** 开关 | `sw<id> [v+,v-] [state,R_on,R_off]` | `SW1 [1,2] [1,1e-6,1e12]` |
| **motor** 直流电机 | `motor<id> [m+,m-] [V_rated,RPM_rated,Ra,La,Kt,J,B]` | `MOTOR1 [1,-1] [12,1000,0.1,0.01,0.05,0.001,0.01]` |
| **xfmr** 变压器 | `xfmr<id> [p1,p2,s1,s2] [L1,Ratio,k]` | `XFMR1 [1,-1,2,-1] [4,2,0.999]` |
| **B** 事件开关 | `B<id> [v+,v-] [事件名,阈值,常开1/常闭0,Ron,Roff]` | `B1 [1,2] [SB1,0.5,1,1e-6,1e12]` |
| **MB** 非自锁按钮 | `MB<id> [v+,v-] [事件名,阈值,常开1/常闭0,Ron,Roff]` | `MB1 [1,2] [BTN,0.5,1,1e-6,1e12]` |
| **RLY** 继电器线圈 | `RLY<id> [c+,c-] [事件名,R_coil,L_coil,I_pullin,I_hold]` | `RLY1 [3,4] [COIL,100,0.01,0.01,0.005]` |
| **VR** 可变电阻 | `VR<id> [v+,v-] [事件名,min_R,max_R]` | `VR1 [1,2] [POT1,100,10000]` |

#### 逻辑域 / Logic

| 元件 | 格式 | 示例 |
|------|------|------|
| **U** 逻辑门 | `U<id> [in1,...,out] [type,V_high]`（0=INV 1=AND 2=NAND 3=OR 4=NOR 5=XOR 6=XNOR） | `U1 [1,2,3] [1,5]` |
| **DFF** D触发器 | `DFF<id> [clk,d,q,nq] [V_high]` | `DFF1 [2,1,3,4] [5]` |
| **JK** JK触发器 | `JK<id> [j,k,clk,q,nq] [V_high]` | `JK1 [1,2,3,4,5] [5]` |
| **T** T触发器 | `T<id> [t,clk,q,nq] [V_high]` | `T1 [1,2,3,4] [5]` |
| **SR** SR锁存器 | `SR<id> [s,r,q,nq] [V_high]` | `SR1 [1,2,3,4] [5]` |
| **CMP** 比较器 | `CMP<id> [vp,vn,vout] [V_high,V_low]` | `CMP1 [1,2,3] [5,0]` |
| **ST** 施密特触发器 | `ST<id> [vin,vout] [V_high,V_low,Vth_high,Vth_low]` | `ST1 [1,5] [5,0,3,1]` |

#### 气路域 / Pneumatic

| 元件 | 格式 | 示例 |
|------|------|------|
| **PS** 气源 | `PS<id> [p+,p-] [压力Pa]` | `PS1 [1,-1] [101325]` |
| **PR** 气阻 | `PR<id> [p1,p2] [阻力Pa·s/m³]` | `PR1 [1,2] [1e8]` |
| **PC** 气容 | `PC<id> [p1] [容量m³/Pa]`（单引脚+大气） | `PC1 [2] [1e-8]` |
| **PL** 气感 | `PL<id> [p1,p2] [惯性Pa·s²/m³]` | `PL1 [4,5] [1e6]` |
| **PCV** 单向阀 | `PCV<id> [p1,p2] [R_on,R_off,V_open]` | `PCV1 [1,4] [1e6,1e12,100]` |

#### 液压域 / Hydraulic

| 元件 | 格式 | 示例 |
|------|------|------|
| **HP** 液压泵 | `HP<id> [h+,h-] [压力Pa]` | `HP1 [1,-1] [10e6]` |
| **HR** 液阻 | `HR<id> [h1,h2] [阻力Pa·s/m³]` | `HR1 [1,2] [1e8]` |
| **HA** 蓄能器 | `HA<id> [h1] [容量m³/Pa]`（单引脚+油箱） | `HA1 [2] [1e-8]` |
| **HL** 液感 | `HL<id> [h1,h2] [惯性Pa·s²/m³]` | `HL1 [4,5] [1e6]` |
| **HCV** 单向阀 | `HCV<id> [h1,h2] [R_on,R_off,V_open]` | `HCV1 [1,4] [1e6,1e12,1e6]` |

#### 跨域传感器 / Cross-Domain Sensors

| 元件 | 格式 | 示例 |
|------|------|------|
| **PSENS** 压力传感器 | `PSENS<id> [sense+,sense-,out+,out-] [gain,offset,domain]` | `PSENS1 [4,-1,5,-1] [1e-4,0,0]` |
| **EP** 电气-气动转换 | `EP<id> [in+,in-,pout+,pout-] [gain,offset]` | `EP1 [1,-1,3,-1] [100000,0]` |
| **EH** 电气-液压转换 | `EH<id> [in+,in-,pout+,pout-] [gain,offset]` | `EH1 [1,-1,3,-1] [1e6,0]` |
| **CS** 电流传感器 | `CS<id> [in+,in-,out+,out-] [gain,offset]` | `CS1 [2,-1,3,-1] [1000,0]` |

### 电压源波形类型 / Voltage Source Waveforms

```
[波形类型, 偏置V, 频率Hz, 相位rad, Vmax, 占空比]
```

| 波形类型 | 说明 | 示例 |
|----------|------|------|
| 0 | DC 直流 | `[0,0,0,0,5]` |
| 1 | AC 正弦波 | `[1,0,50,0,5,0.5]` |
| 2 | 方波 | `[2,0,100,0,5,0.5]` |
| 3 | 三角波 | `[3,0,50,0,5,0.5]` |
| 4 | 锯齿波 | `[4,0,100,0,5,0.5]` |
| 5 | 脉冲波 | `[5,0,1000,0,5,0.5]` |
| 6 | 噪声 | `[6,0,100,0,1,0]` |

### 事件开关、非自锁按钮与继电器 / Event Switch, Momentary Button & Relay Coil

事件开关(B)通过 TUI `set` 命令实时控制通断，状态保持直到下一次 set。
非自锁按钮(MB)与事件开关(B)语法相同，但触发后自动复位——每按一次只闭合一个时间步，
适合真实按钮的脉冲行为。继电器线圈(RLY)是**生产者**元件，吸合后自动将状态回写到事件系统，
实现自锁闭环控制。可变电阻(VR)将事件值 0~1 线性映射到 min_R~max_R。

| 参数 | B 事件开关 | RLY 继电器线圈 |
|------|-----------|---------------|
| 事件名 | eventName | eventName（可多个通道） |
| 阈值/电阻 | threshold | R_coil（线圈电阻 Ω） |
| 模式/电感 | normallyOpen(1常开/0常闭) | L_coil（线圈电感 H） |
| Ron/I_pullin | R_on（导通电阻 Ω） | I_pullin（吸合电流 A） |
| Roff/I_hold | R_off（断开电阻 Ω） | I_hold（保持电流 A，< I_pullin） |

### 潮流网表 (B/BR) / Power Flow Netlist

`29_powerflow_3bus.net` 使用潮流专用语法（非元件网表），CLI 自动识别并求解：

```
B<id> slack|pv|pq [V=..] [Theta=..] [P=..] [Q=..] [Qmin=..] [Qmax=..]   # 母线（标幺值, Theta 单位度）
BR<id> From-To [R=..] [X=..] [B=..] [Tap=..]                            # 支路
```

```
B1 slack V=1.05
B2 pv   V=1.0 P=0.5 Qmin=-0.3 Qmax=0.3
B3 pq   P=-1.0 Q=-0.5
BR1 1-2 R=0.01 X=0.1
BR2 1-3 R=0.01 X=0.1
BR3 2-3 R=0.01 X=0.1
```

详细说明与输出格式见 [docs/powerflow.md](../../docs/powerflow.md) 第 3 节。

### 子电路语法 / Subcircuit Syntax

```
.subckt <name> <port1> <port2> ...
  R1 [port1,mid] [1000]
  C1 [mid,port2] [1e-6]
.ends <name>

X1 [ext_node1,ext_node2] [name]
```
