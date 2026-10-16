// slots.go 定义串口桥消费的 K2 UART 槽表索引与宿主侧最小接口 SlotIo。
//
// 槽表来自批 1 冻结契约 K2（runs/ttl-uart-components/50-subtasks/m2.md:28-37），
// 桥只读该表、不定义新槽：槽不足时必须上报「契约缺口 K2」，不得自行加槽。
// 入站事件 "<实例名>.TX" → 槽 3；ACK（槽 1 上的事件名）→ 槽 15；
// 出站直读槽 11/12/13；同名事件在一步内只保留最后一个值 ⇒ 桥必须一次一字节 + 确认/推进（C15）。
package serialbridge

// K2 槽表常量（值即槽索引；bool 槽按非 0 为真）。
const (
	SlotBaud       = 0  // baud：波特率（float64，只作为参数，不用于断言——C13）
	SlotAckEvent   = 1  // ackEvent：ACK 事件名所在的 string 槽（桥按配置使用，不读该槽名字）
	SlotVHigh      = 2  // VHigh：逻辑高电平
	SlotTxByte     = 3  // txByte：入站事件 "<实例名>.TX" 的目标槽，float64(0..255)
	SlotTxShifter  = 4  // txShifter：发送移位器
	SlotTxBit      = 5  // txBit：发送位序号
	SlotTxPhase    = 6  // txPhase：发送相位
	SlotLastTxTime = 7  // lastTxTime：最近一次发送时刻
	SlotRxShifter  = 8  // rxShifter：接收移位器
	SlotRxBit      = 9  // rxBit：接收位序号
	SlotRxPhase    = 10 // rxPhase：接收相位
	SlotRxByte     = 11 // rxByte：出站字节（桥直读本槽）
	SlotDataReady  = 12 // dataReady：非 0 表示有可读字节
	SlotFrameError = 13 // frameError：非 0 表示当前字节帧错误
	SlotLastRxTime = 14 // lastRxTime：最近一次接收时刻
	SlotAckFlag    = 15 // ackFlag：ACK 事件目标槽；非 0 ⇒ 本步内 dataReady=false、ackFlag=0
	SlotCount      = 16 // 槽总数（越界判定的边界）
)

// SlotIo 是桥对仿真引擎的最小依赖面：读槽 / 写事件 / 推进仿真。
// 桥不 import element/**（K2 的 element/serial 由批 2 提供，本包不依赖其类型）；
// 真实适配器由集成方按本接口实现，本包测试用 _test.go 内的内存假实现驱动。
type SlotIo interface {
	// ReadSlot 读取槽 idx 的数值；bool 槽按 true=1/false=0 返回。
	// 槽索引越界、槽值为 nil 或类型不是数值时返回错误（不得返回 0 冒充数据）。
	ReadSlot(idx int) (float64, error)
	// WriteEvent 把宿主事件写入引擎（等价 element.Context.SetEvent）：
	// 同名事件在一步内只保留最后一个值，直到 Advance 在步首消费它。
	WriteEvent(name string, value float64) error
	// Advance 推进一个仿真步，并返回本步被引擎消费（PushEvents 取出后置 nil）的事件名。
	// 桥用该返回值判定入站字节是否真的被取走：不在返回集合内 ⇒ 仍在途（背压）。
	Advance() ([]string, error)
}
