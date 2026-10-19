// doc.go：element/serial 包的职责与冻结契约入口说明。
package serial

// Package serial 提供 TTL 电平串口元件：网表类型 UART（NodeType 48），8N1 帧、
// 波特率可配、INTR 高有效电平输出、事件入站与 ACK 清除。
//
// 契约（K2，冻结；槽位表见 uartSlots.go）：
//   - 网表: UART<id> [tx, rx, intr] [baud, ackEvent, VHigh]；三根引脚均 PinBoolean，
//     与既有逻辑域（element/logic）同域；Base 按 len(Pins) 成对切换形态：
//     3 针 = 3 引脚/2 电压源（tx、intr）；13 针 = 13 引脚/11 电压源，
//     脚位固定 0=tx、1=rx、2=intr、3..10=D0..D7、11=RDY、12=ACK（ACK 是输入，不配电压源）；
//   - 13 针字节口（K1）：槽 3 的字节取走后锁存到 D0..D7 并拉起 RDY；加载器拉高 ACK 后
//     UART 拉低 RDY（槽 22=1 记录「已 ACK」）；ACK 回低且 RDY 已低之前不得取下一字节。
//     3 针实例不分配 D/RDY 电压源、不加 ACK 门控，行为与改动前逐位一致；
//   - 入站字节事件 "<实例名>.TX" → 槽 3（float64，0..255）；
//     ACK 事件（槽 1，网表未给时默认 "<实例名>.ACK"）→ 槽 15（float64，非 0 即确认）；
//   - 槽 12 == true ⇒ intr 钳位 VHigh，否则 0V；槽 15 非 0 ⇒ 本步内 dataReady=false、ackFlag=0；
//   - 事件落点槽 3/15 不进 OrigValue，其余运行态槽全部进 OrigValue，Reset 复位全部运行态槽；
//   - 比特周期 T=1/baud，采样点在起始沿后 (1.5+n)·T（n=0..7 数据位）、9.5·T（停止位），
//     停止位采样为低即 frameError=true。
//
// 本包不调用 SetTriggers、不 import circuit/element/time，也不 import circuit/element/register
// （注册表中的 import 行由 Lead 集成时写）；时序精度依赖调用方显式 `--max-step`（≤ T/10）
// 加元件侧 digital.StepGuard 自检。
