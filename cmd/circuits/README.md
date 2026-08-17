# Circuit Test Netlists / 电路测试网表

本目录包含用于测试和演示 `circuit` 电路仿真器的网表示例文件。

This directory contains example netlist files for testing and demonstrating the `circuit` circuit simulator.

---

## 文件列表 / File List

| 文件 | 描述 | Description |
|------|------|-------------|
| `01_resistor_divider.net` | 电阻分压器 — 最简单的纯电阻电路 | Resistor divider — simplest resistive circuit |
| `02_rc_filter.net` | RC 低通滤波器 — 含储能元件，展示瞬态响应 | RC low-pass filter — energy storage element, transient response |
| `03_diode_rectifier.net` | 二极管半波整流器 — 含非线性元件 | Diode half-wave rectifier — nonlinear element |
| `04_transistor_switch.net` | NPN 三极管开关 — 含非线性元件 | NPN transistor switch — nonlinear element |
| `05_opamp_amplifier.net` | 运放同相放大器 — 含非线性元件 | Op-amp non-inverting amplifier — nonlinear element |
| `06_voltage_sources.net` | 多种电压源波形演示 — 5 种波形 | Voltage source waveform demo — 5 waveforms |
| `07_rlc_circuit.net` | RLC 串联振荡电路 — 含 R、L、C | RLC series oscillator — R, L, C |
| `08_subcircuit.net` | 子电路示例 — RC 滤波器级联 | Subcircuit example — cascaded RC filters |
| `09_half_adder.net` | RTL 半加器 — 8-NOR 门数字逻辑 | RTL half adder — 8-NOR gate digital logic |
| `10_latch_self_hold.net` | 自锁启停电路 — 继电器自保持控制 | Self-holding latch circuit — relay self-hold control |

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
| `--format` | csv | 输出格式: `csv`, `tsv`, `table` |
| `--quiet` | false | 静默模式，不输出元信息到 stderr |
| `-h`, `--help` | - | 显示帮助 |

用法: `circuit [选项] <网表文件>`

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

| 元件 | 格式 | 示例 |
|------|------|------|
| **R** 电阻 | `R<id> [v+,v-] [阻值Ω]` | `R1 [1,-1] [1000]` |
| **C** 电容 | `C<id> [v+,v-] [容值F]` | `C1 [2,-1] [1e-6]` |
| **L** 电感 | `L<id> [v+,v-] [感值H]` | `L1 [1,2] [1e-3]` |
| **D** 二极管 | `D<id> [anode,cathode] [Is,Vz,N,Rs,T]`（方向由引脚顺序决定：anode→cathode 正向） | `D1 [1,2] [1e-14,0]` |
| **Q** NPN三极管 | `Q<id> [base,collector,emitter] [is_PNP,betβ]` | `Q1 [2,3,-1] [false,100]` |
| **V** 电压源 | `V<id> [v+,v-] [波形,bias,频率,相位,Vmax,duty]` | 见下方 |
| **I** 电流源 | `I<id> [v+,v-] [电流A]` | `I1 [1,-1] [0.01]` |
| **opamp** 运放 | `opamp<id> [noninv+,inv-,out] [V+,V-,开环增益]` | `opamp1 [1,2,3] [15,-15,1e5]` |
| **B** 事件开关 | `B<id> [v+,v-] [事件名,阈值,常开1/常闭0,Ron,Roff]` | `B1 [1,2] [SB1,0.5,1,1e-6,1e12]` |
| **MB** 非自锁按钮 | `MB<id> [v+,v-] [事件名,阈值,常开1/常闭0,Ron,Roff]` | `MB1 [1,2] [BTN,0.5,1,1e-6,1e12]` |
| **RLY** 继电器线圈 | `RLY<id> [c+,c-] [事件名,R_coil,L_coil,I_pullin,I_hold]` | `RLY1 [3,4] [COIL,100,0.01,0.01,0.005]` |

### 电压源波形类型 / Voltage Source Waveforms

```
[波形类型, 偏置V, 频率Hz, 相位°, Vmax, 占空比]
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
实现自锁闭环控制。

| 参数 | B 事件开关 | RLY 继电器线圈 |
|------|-----------|---------------|
| 事件名 | eventName | eventName（可多个通道） |
| 阈值/电阻 | threshold | R_coil（线圈电阻 Ω） |
| 模式/电感 | normallyOpen(1常开/0常闭) | L_coil（线圈电感 H） |
| Ron/I_pullin | R_on（导通电阻 Ω） | I_pullin（吸合电流 A） |
| Roff/I_hold | R_off（断开电阻 Ω） | I_hold（保持电流 A，< I_pullin） |

### 子电路语法 / Subcircuit Syntax

```
.subckt <name> <port1> <port2> ...
  R1 [port1,mid] [1000]
  C1 [mid,port2] [1e-6]
.ends <name>

X1 [ext_node1,ext_node2] [name]
```
