# 元件参考手册

基于 `element/register` 实际注册目录（46 种元件类型，可通过 MCP `circuit_list_component_types` 查询）。
节点 `-1` 为全局地（电气地 0V / 气路大气压 / 油路油箱），**节点 `0` 是普通节点，不是地**。

## 元件特性标记 (Flags)

每个元件通过 `Config.Flags` 位掩码标记其特性，检测方式：`config.IsFlag(flag Flag) bool`

| Flag | 值 | 含义 | 标记元件 |
|------|-----|------|---------|
| `FlagReactive` | `0x02` | 储能元件，影响步长自适应策略 | C, L, PC, PL, HA, HL, RLY |
| `FlagNonlinear` | `0x04` | 非线性元件，需 Newton-Raphson 迭代 | D, Q, Z, LED, M, J, DFF, JK, T, SR, U, S |
| `FlagCacheStamp` | `0x08` | DoStep 仅依赖引脚电压，可启用盖章缓存 | D, Q, Z, LED |

## 元件目录总览（46 种）

| 元件 | 名称 | 域 | 引脚 | 关键参数 |
|------|------|----|------|---------|
| R | 电阻 | 电气 | `[v+, v-]` | R |
| C | 电容 | 电气 | `[v+, v-]` | C |
| L | 电感 | 电气 | `[v+, v-]` | L |
| D | 二极管 | 电气 | `[anode, cathode]` | Is, Vz, N, Rs, T |
| Q | 三极管 | 电气 | `[base, collector, emitter]` | PNP, hFE |
| V | 电压源 | 电气 | `[v+, v-]` | waveform, bias, frequency, phase, V_max, duty_cycle |
| I | 电流源 | 电气 | `[v+, v-]` | I |
| opamp | 运算放大器 | 电气 | `[Vp, Vn, Vout]` | Vmax, Vmin, G |
| E | VCVS 压控电压源 | 电气 | `[cp, cn, op, on]` | gain |
| G | VCCS 压控电流源 | 电气 | `[cp, cn, op, on]` | gm |
| F | CCCS 流控电流源 | 电气 | `[cp, cn, op, on]` | gain |
| H | CCVS 流控电压源 | 电气 | `[cp, cn, op, on]` | rm |
| Z | 齐纳二极管 | 电气 | `[anode, cathode]` | Is, Vz, N, Rs, T |
| LED | 发光二极管 | 电气 | `[anode, cathode]` | Is, Vz, N, Rs, T |
| M | MOSFET | 电气 | `[drain, gate, source]` | type, VTO, KP, LAMBDA |
| J | JFET | 电气 | `[drain, gate, source]` | type, VTO, BETA, LAMBDA |
| sw | 开关 | 电气 | `[v+, v-]` | state, R_on, R_off |
| B | 事件开关 | 电气 | `[v+, v-]` | eventName, threshold, normallyOpen, R_on, R_off |
| MB | 非自锁按钮 | 电气 | `[v+, v-]` | eventName, threshold, normallyOpen, R_on, R_off |
| VR | 事件可变电阻 | 电气 | `[v+, v-]` | eventName, min_R, max_R |
| RLY | 继电器线圈 | 电气 | `[c+, c-]` | eventName, R_coil, L_coil, I_pullin, I_hold |
| motor | 直流电机 | 电气 | `[m+, m-]` | V_rated, RPM_rated, Ra, La, Kt, J, B |
| xfmr | 变压器 | 电气 | `[p1, p2, s1, s2]` | L1, Ratio, k |
| U | 逻辑门 | 逻辑 | `[in1.., out]` | type, V_high |
| S | 通用逻辑门 | 逻辑 | `[in0.., out0..]` | type, n_inputs, n_outputs, V_high |
| DFF | D 触发器 | 逻辑 | `[clk, d, q, nq]` | V_high |
| JK | JK 触发器 | 逻辑 | `[j, k, clk, q, nq]` | V_high |
| T | T 触发器 | 逻辑 | `[t, clk, q, nq]` | V_high |
| SR | SR 锁存器 | 逻辑 | `[s, r, q, nq]` | V_high |
| CMP | 比较器 | 电气 | `[vp, vn, vout]` | V_high, V_low |
| ST | 施密特触发器 | 电气 | `[vin, vout]` | V_high, V_low, Vth_high, Vth_low |
| PS | 气源 | 气路 | `[p+, p-]` | pressure |
| PR | 气阻 | 气路 | `[p1, p2]` | R |
| PC | 气容 | 气路 | `[p1]` | C（单引脚 + 大气） |
| PL | 气感 | 气路 | `[p1, p2]` | L |
| PCV | 单向阀 | 气路 | `[p1, p2]` | R_on, R_off, V_open |
| HP | 液压泵 | 液压 | `[h+, h-]` | pressure |
| HR | 液阻 | 液压 | `[h1, h2]` | R |
| HA | 蓄能器 | 液压 | `[h1]` | C（单引脚 + 油箱） |
| HL | 液感 | 液压 | `[h1, h2]` | L |
| HCV | 单向阀 | 液压 | `[h1, h2]` | R_on, R_off, V_open |
| PSENS | 压力传感器 | 气/液→电气 | `[sense+, sense-, out+, out-]` | gain, offset, domain |
| EP | 电气-气动转换器 | 电气→气路 | `[in+, in-, pout+, pout-]` | gain, offset |
| EH | 电气-液压转换器 | 电气→液压 | `[in+, in-, pout+, pout-]` | gain, offset |
| CS | 电流传感器 | 电气 | `[in+, in-, out+, out-]` | gain, offset |
| X | 子电路实例 | — | 动态 | subckt |

---

## 电气域元件

### 电阻 R

| 项目 | 说明 |
|------|------|
| 引脚 | `[v+, v-]` 2 个 |
| 参数 | `[R]` — 阻值 (Ω)，默认 10000 |
| 示例 | `R1 [1,-1] [1000]` — 1kΩ 电阻，节点 1→地 |

### 电容 C

| 项目 | 说明 |
|------|------|
| 引脚 | `[v+, v-]` 2 个 |
| 参数 | `[C]` — 电容值 (F)，默认 1e-6 |
| 示例 | `C1 [2,-1] [1e-6]` — 1μF 电容 |
| 算法 | 伴随模型（梯形积分）；FlagReactive |

### 电感 L

| 项目 | 说明 |
|------|------|
| 引脚 | `[v+, v-]` 2 个 |
| 参数 | `[L]` — 电感值 (H)，默认 1e-3 |
| 示例 | `L1 [1,2] [1e-3]` — 1mH 电感 |
| 算法 | 伴随模型（梯形 / 后向欧拉）；FlagReactive |

### 二极管 D

| 项目 | 说明 |
|------|------|
| 引脚 | `[anode, cathode]` 2 个（方向由引脚顺序决定：anode→cathode 正向） |
| 参数 | `[Is, Vz, N, Rs, T]` — 饱和电流(A)、齐纳电压(V, 0=无击穿)、发射系数、串联电阻(Ω)、温度(K) |
| 示例 | `D1 [1,2] [1e-14,0]` — 普通二极管 Is=1e-14A |
| 算法 | SPICE 二极管模型 + limitStep + 加权贡献；FlagNonlinear + FlagCacheStamp |

### 三极管 Q

| 项目 | 说明 |
|------|------|
| 引脚 | `[base, collector, emitter]` 3 个 |
| 参数 | `[PNP, hFE]` — PNP 标志 (bool), 电流增益 (float) |
| 示例 | `Q1 [2,3,-1] [false,100]` — NPN, β=100, B=2, C=3, E=-1(GND) |
| 算法 | Gummel-Poon 类模型；FlagNonlinear + FlagCacheStamp |

**半加器中的用法**：每个 RTL NOR 门使用 2 个 NPN 三极管（β=100）+ 1 个集电极电阻（1kΩ）+ 2 个基极电阻（10kΩ）。

### 电压源 V

| 引脚 | `[v+, v-]` 2 个 |
|------|------|

#### 7 种波形

| waveform | 名称 | 公式 | 适用场景 |
|----------|------|------|---------|
| `0` | DC 直流 | `V_max + bias` | 电源偏置 |
| `1` | AC 交流 | `sin(ωt + phase) × V_max + bias` | 正弦信号 |
| `2` | 方波 (SQUARE) | `if(ωt > 2π·duty) bias-V_max else bias+V_max` | — |
| `3` | 三角波 | `tri(ωt) × V_max + bias` | 三角信号 |
| `4` | 锯齿波 | `(mod(ωt,2π)/(π) - 1) × V_max + bias` | 扫描信号 |
| **`5`** | **脉冲波 (PULSE)** | **`if(ωt < 2π·duty) V_max+bias else bias`** | **RTL 数字输入** |
| `6` | 噪声 | 正态分布随机值 | 噪声分析 |

#### 参数

```
[waveform, bias, frequency, phase, V_max, duty_cycle, t_zero, noise]
```

| 序号 | 名称 | 默认值 | 说明 |
|------|------|--------|------|
| 0 | waveform | `0` (DC) | 波形类型枚举 |
| 1 | bias | `0` | 偏置电压 (V) |
| 2 | frequency | `0` | 频率 (Hz) |
| 3 | phase | `0` | 相位偏移 (rad) |
| 4 | V_max | `5` | 最大电压 (V) |
| 5 | duty_cycle | `0` | 占空比 (0–1) |
| 6 | t_zero | `0` | 时间零点 |
| 7 | noise | `0` | 噪声值（自动生成） |

#### 脉冲波使用注意

**RTL 三极管逻辑电路必须使用 `WfPULSE` (waveform=5)**，因为：
- 脉冲波输出 `V_max`（高电平）或 `bias`（低电平=0V）
- 方波 (`WfSQUARE`, waveform=2) 输出 `bias ± V_max`，低电平为 -5V，会导致三极管错误关断

**脉冲波配置示例**：

```
V1 [10,-1] [5,0,20,0,5,0.5]
# waveform=5(PULSE), bias=0V, freq=20Hz, phase=0, V_max=5V, duty=0.5
# → 输出 5V(高) / 0V(低) 的 20Hz 方波
```

### 电流源 I

| 项目 | 说明 |
|------|------|
| 引脚 | `[v+, v-]` 2 个 |
| 参数 | `[I]` — 电流值 (A)，默认 0.01 |
| 示例 | `I1 [-1,1] [0.01]` — 10mA 电流源（从地注入节点 1） |

### 运算放大器 opamp

| 项目 | 说明 |
|------|------|
| 引脚 | `[Vp, Vn, Vout]` — 同相输入、反相输入、输出 |
| 参数 | `[Vmax, Vmin, G]` — 输出上限/下限 (V)、开环增益 |
| 示例 | `opamp1 [1,2,3] [15,-15,1e5]` |

### 受控源 E / G / F / H

四类线性受控源，引脚均为 `[控制+, 控制-, 输出+, 输出-]`：

| 元件 | 名称 | 参数 | 示例 |
|------|------|------|------|
| E | VCVS 压控电压源 | `[gain]` 电压增益 | `E1 [1,-1,2,-1] [2]` |
| G | VCCS 压控电流源 | `[gm]` 跨导 (S) | `G1 [3,-1,4,-1] [0.001]` |
| F | CCCS 流控电流源 | `[gain]` 电流增益 | `F1 [6,-1,8,-1] [2]` |
| H | CCVS 流控电压源 | `[rm]` 跨阻 (Ω) | `H1 [10,-1,12,-1] [1000]` |

### 齐纳二极管 Z

| 项目 | 说明 |
|------|------|
| 引脚 | `[anode, cathode]` 2 个 |
| 参数 | `[Is, Vz, N, Rs, T]` — Vz 默认 5.1V |
| 示例 | `Z1 [-1,2] [1e-14,5.1,1,0.1]` — 阳极接地，阴极输出 5.1V 稳压 |
| 算法 | 二极管模型 + 齐纳击穿分支；FlagNonlinear + FlagCacheStamp |

### 发光二极管 LED

| 项目 | 说明 |
|------|------|
| 引脚 | `[anode, cathode]` 2 个 |
| 参数 | `[Is, Vz, N, Rs, T]` — 低 Is 默认 1e-18，N=1.5 |
| 示例 | `LED1 [2,-1] [1e-18,0,1.5,5]` |
| 算法 | 二极管模型（低饱和电流高导通电压）；FlagNonlinear + FlagCacheStamp |

### MOSFET M

| 项目 | 说明 |
|------|------|
| 引脚 | `[drain, gate, source]` 3 个 |
| 参数 | `[type, VTO, KP, LAMBDA]` — type 0=NMOS/1=PMOS, 阈值电压(V), 跨导系数(A/V²), 沟道长度调制(1/V) |
| 示例 | `M1 [2,3,-1] [0,1.5,2e-5,0.01]` — NMOS, VTO=1.5V |
| 算法 | Shichman-Hodges 模型 + Newton-Raphson 线性化；FlagNonlinear |

### JFET J

| 项目 | 说明 |
|------|------|
| 引脚 | `[drain, gate, source]` 3 个 |
| 参数 | `[type, VTO, BETA, LAMBDA]` — type 0=NJF/1=PJF |
| 示例 | `J1 [2,3,-1] [0,2,1e-4,0.02]` |
| 算法 | 平方律模型；FlagNonlinear |

### 开关 sw

| 项目 | 说明 |
|------|------|
| 引脚 | `[v+, v-]` 2 个 |
| 参数 | `[state, R_on, R_off]` — state 1=闭合/0=断开（网表静态配置） |
| 示例 | `SW1 [1,2] [1,1e-6,1e12]` — 闭合开关 |

### 事件开关 B

| 项目 | 说明 |
|------|------|
| 引脚 | `[v+, v-]` 2 个 |
| 参数 | `[eventName, threshold, normallyOpen, R_on, R_off]` — 事件名、阈值、1常开/0常闭、导通/断开电阻 |
| 示例 | `B1 [1,2] [SB1,0.5,1,1e-6,1e12]` |
| 说明 | 事件驱动按钮/触点，运行中经 `circuit_set_event` 或 TUI `set` 实时控制 |

### 非自锁按钮 MB

| 项目 | 说明 |
|------|------|
| 引脚 | `[v+, v-]` 2 个 |
| 参数 | `[eventName, threshold, normallyOpen, R_on, R_off, holdSteps]` |
| 示例 | `MB1 [1,2] [BTN,0.5,1,1e-6,1e12]` |
| 说明 | 与 B 语法相同，但触发后自动复位（只闭合 holdSteps 个时间步），模拟真实按钮脉冲 |

### 事件可变电阻 VR

| 项目 | 说明 |
|------|------|
| 引脚 | `[v+, v-]` 2 个 |
| 参数 | `[eventName, min_R, max_R]` — 事件值 0~1 线性映射到 min_R~max_R |
| 示例 | `VR1 [1,2] [POT1,100,10000]` |

### 继电器线圈 RLY

| 项目 | 说明 |
|------|------|
| 引脚 | `[c+, c-]` 2 个 |
| 参数 | `[eventName, R_coil, L_coil, I_pullin, I_hold]` — 线圈电阻(Ω)、电感(H)、吸合/保持电流(A) |
| 示例 | `RLY1 [3,4] [COIL,100,0.01,0.01,0.005]` |
| 说明 | **生产者**元件：吸合后自动把状态回写事件系统，实现自锁闭环（见 10_latch_self_hold.net）；RL 串联梯形积分模型 + 磁滞；FlagReactive |

### 直流电机 motor

| 项目 | 说明 |
|------|------|
| 引脚 | `[m+, m-]` 2 个 |
| 参数 | `[V_rated, RPM_rated, Ra, La, Kt, J, B]` — 额定电压/转速、电枢电阻/电感、转矩常数、转动惯量、阻尼 |
| 示例 | `MOTOR1 [1,-1] [12,1000,0.1,0.01,0.05,0.001,0.01]` |
| 算法 | 反电动势模型（电枢 + 机械动力学） |

### 变压器 xfmr

| 项目 | 说明 |
|------|------|
| 引脚 | `[p1, p2, s1, s2]` — 初级 + 次级 |
| 参数 | `[L1, Ratio, k]` — 初级电感(H)、匝数比、耦合系数(默认 0.999) |
| 示例 | `XFMR1 [1,-1,2,-1] [4,2,0.999]` — 次级按 2:1 升压 |

---

## 逻辑域元件

### 逻辑门 U

| 项目 | 说明 |
|------|------|
| 引脚 | `[in1, in2, ..., out]` — N 个输入 + 1 个输出（输入数由引脚列表决定） |
| 参数 | `[type, V_high]` — 门类型 (int), 高电平电压 (V) |

| type | 名称 | 说明 |
|------|------|------|
| `0` | INV | 非门 |
| `1` | AND | 与门 |
| `2` | NAND | 与非门 |
| `3` | OR | 或门 |
| `4` | NOR | 或非门 |
| `5` | XOR | 异或门 |
| `6` | XNOR | 同或门 |

示例：
```
U1 [1,2,3] [1,5]   # AND 门, 输入=1,2, 输出=3, V_high=5V
U2 [-1,-1,5] [0,5] # NOT 门, 输入=-1(0), 输出=5, V_high=5V
U3 [1,6,8] [3,5]   # OR 门
U4 [1,2,11] [5,5]  # XOR 门
```

### 通用逻辑门 S

| 项目 | 说明 |
|------|------|
| 引脚 | `[in0.., out0..]` — N 输入 + M 输出 |
| 参数 | `[type, n_inputs, n_outputs, V_high]` — 可配置多输入多输出版本 |

### D 触发器 DFF

| 项目 | 说明 |
|------|------|
| 引脚 | `[clk, d, q, nq]` — 时钟、数据、同相/反相输出 |
| 参数 | `[V_high]` |
| 示例 | `DFF1 [2,1,3,4] [5]` |
| 说明 | 时钟上升沿采样 D 输入；FlagNonlinear |

### JK 触发器 JK

| 项目 | 说明 |
|------|------|
| 引脚 | `[j, k, clk, q, nq]` |
| 参数 | `[V_high]` |
| 说明 | 边沿触发；FlagNonlinear |

### T 触发器 T

| 项目 | 说明 |
|------|------|
| 引脚 | `[t, clk, q, nq]` |
| 参数 | `[V_high]` |
| 说明 | 边沿触发翻转；FlagNonlinear |

### SR 锁存器 SR

| 项目 | 说明 |
|------|------|
| 引脚 | `[s, r, q, nq]` |
| 参数 | `[V_high]` |
| 说明 | 电平触发；FlagNonlinear |

### 比较器 CMP

| 项目 | 说明 |
|------|------|
| 引脚 | `[vp, vn, vout]` — 同相/反相输入、输出 |
| 参数 | `[V_high, V_low]` — 输出高/低电平 |
| 示例 | `CMP1 [1,2,3] [5,0]` |

### 施密特触发器 ST

| 项目 | 说明 |
|------|------|
| 引脚 | `[vin, vout]` |
| 参数 | `[V_high, V_low, Vth_high, Vth_low]` — 输出电平 + 上/下阈值（滞回） |
| 示例 | `ST1 [1,5] [5,0,3,1]` |

---

## 气路域元件

压力↔电压类比，复用电路元件 MNA 模型；节点 `-1` = 大气压。

| 元件 | 引脚 | 参数 | 示例 |
|------|------|------|------|
| **PS** 气源 | `[p+, p-]` | `[pressure]` 压力(Pa) | `PS1 [1,-1] [101325]` — 1atm |
| **PR** 气阻 | `[p1, p2]` | `[R]` 阻力(Pa·s/m³) | `PR1 [1,2] [1e8]` |
| **PC** 气容 | `[p1]` | `[C]` 容量(m³/Pa)，单引脚+大气 | `PC1 [2] [1e-8]`；FlagReactive |
| **PL** 气感 | `[p1, p2]` | `[L]` 惯性(Pa·s²/m³) | `PL1 [4,5] [1e6]`；FlagReactive |
| **PCV** 单向阀 | `[p1, p2]` | `[R_on, R_off, V_open]` | `PCV1 [1,4] [1e6,1e12,100]` |

## 液压域元件

| 元件 | 引脚 | 参数 | 示例 |
|------|------|------|------|
| **HP** 液压泵 | `[h+, h-]` | `[pressure]` 压力(Pa) | `HP1 [1,-1] [10e6]` — 10MPa |
| **HR** 液阻 | `[h1, h2]` | `[R]` 阻力(Pa·s/m³) | `HR1 [1,2] [1e8]` |
| **HA** 蓄能器 | `[h1]` | `[C]` 容量(m³/Pa)，单引脚+油箱 | `HA1 [2] [1e-8]`；FlagReactive |
| **HL** 液感 | `[h1, h2]` | `[L]` 惯性(Pa·s²/m³) | `HL1 [4,5] [1e6]`；FlagReactive |
| **HCV** 单向阀 | `[h1, h2]` | `[R_on, R_off, V_open]` | `HCV1 [1,4] [1e6,1e12,1e6]` |

---

## 跨域传感器与转换器

| 元件 | 引脚 | 参数 | 示例 |
|------|------|------|------|
| **PSENS** 压力传感器 | `[sense+, sense-, out+, out-]`（气/液压侧 + 电气侧） | `[gain, offset, domain]` gain 默认 1e-5 V/Pa | `PSENS1 [4,-1,5,-1] [1e-4,0,0]` |
| **EP** 电气-气动 | `[in+, in-, pout+, pout-]` | `[gain, offset]` gain 默认 100000 Pa/V | `EP1 [1,-1,3,-1] [100000,0]` |
| **EH** 电气-液压 | `[in+, in-, pout+, pout-]` | `[gain, offset]` gain 默认 1e6 Pa/V | `EH1 [1,-1,3,-1] [1e6,0]` |
| **CS** 电流传感器 | `[in+, in-, out+, out-]`（支路侧 + 电压侧） | `[gain, offset]` gain 默认 1 V/A | `CS1 [2,-1,3,-1] [1000,0]` |

- PSENS 的 `domain` 参数决定 sense 侧引脚类型：0=气路、1=液压（默认气路）
- 所有传感器使用手动构建 `[]Pin` 数组实现混合引脚类型（跨域元件两侧引脚连接不同节点，互不冲突）

---

## 子电路 X

| 项目 | 说明 |
|------|------|
| 语法 | `.subckt <name> <port1> ...` 定义，`X<id> [引脚...] [子电路名]` 实例化 |
| 示例 | 见 08_subcircuit.net（3 级 RC 级联） |
| 注意 | 稳态分析（AC/小信号）不支持子电路 |
