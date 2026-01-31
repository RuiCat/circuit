package vm

import "encoding/binary"

// Memory 内存封装
type Memory struct {
	Data         []byte
	VmMemorySize uint32 // 主内存大小
	extram       []byte // 挂载的外部扩展内存。
	extramLen    uint32 // 外部扩展内存的长度。
	extramDirty  bool   // 标记扩展内存是否被写入。
}

// Load 加载内存
func (memory *Memory) Load(rom []byte) {
	copy(memory.Data[:], rom)
}

// GetMemory 返回一个指向虚拟机主内存的字节切片。
func (memory *Memory) GetMemory() []byte {
	return memory.Data[:]
}
func (memory *Memory) LoadUint8(addr uint32) uint8 {
	return memory.Data[addr]
}
func (memory *Memory) LoadUint32(addr uint32) uint32 {
	return binary.LittleEndian.Uint32(memory.Data[addr:])
}
func (memory *Memory) LoadUint16(addr uint32) uint16 {
	return binary.LittleEndian.Uint16(memory.Data[addr:])
}
func (memory *Memory) LoadUint64(addr uint32) uint64 {
	return binary.LittleEndian.Uint64(memory.Data[addr:])
}
func (memory *Memory) PutUint32(addr uint32, v uint32) {
	binary.LittleEndian.PutUint32(memory.Data[addr:], v)
}
func (memory *Memory) PutUint64(addr uint32, v uint64) {
	binary.LittleEndian.PutUint64(memory.Data[addr:], v)
}
func (memory *Memory) PutUint16(addr uint32, v uint16) {
	binary.LittleEndian.PutUint16(memory.Data[addr:], v)
}
func (memory *Memory) PutUint8(addr uint32, v uint8) {
	memory.Data[addr] = v
}

// ExtramDirty 返回一个布尔值，指示扩展内存自上次检查以来是否已被写入。
func (memory *Memory) ExtramDirty() bool {
	return memory.extramDirty
}

// GetSafePtr 根据给定的地址和长度，安全地从主内存或扩展内存中获取一个字节切片。
// 它会进行边界检查，如果访问越界，则返回错误并设置虚拟机状态。
func (memory *Memory) GetSafePtr(addr, length uint32) ([]byte, bool) {
	if VmEetRamBase <= addr && addr < 0x12000000 {
		if memory.extram == nil {
			return nil, false
		}
		ptrstart := addr - VmEetRamBase
		if ptrstart > memory.extramLen || ptrstart+length > memory.extramLen {
			return nil, false
		}
		return memory.extram[ptrstart : ptrstart+length], true
	}
	ptrstart := addr - VmRamImageOffSet
	if ptrstart > memory.VmMemorySize || ptrstart+length > memory.VmMemorySize {
		return nil, false
	}
	return memory.Data[ptrstart : ptrstart+length], true
}

// Extram 将一个外部字节切片挂载为虚拟机的扩展内存。
func (memory *Memory) Extram(ram []byte) {
	memory.extram = ram
	memory.extramLen = uint32(len(ram))
}

// ExtramLoad 根据指定的访问类型（字节、半字、字）从扩展内存中读取数据。
// 它处理地址转换和边界检查。
func (memory *Memory) ExtramLoad(addr uint32, accessTyp uint32) (uint32, VmErr) {
	if memory.extram == nil {
		return 0, VmErrNone
	}
	addr -= VmEetRamBase
	if addr >= memory.extramLen {
		return 0, VmErrMemRd
	}
	switch accessTyp {
	case 0: // LB
		return uint32(int8(memory.extram[addr])), VmErrNone
	case 1: // LH
		return uint32(int16(binary.LittleEndian.Uint16(memory.extram[addr:]))), VmErrNone
	case 2: // LW
		return binary.LittleEndian.Uint32(memory.extram[addr:]), VmErrNone
	case 4: // LBU
		return uint32(memory.extram[addr]), VmErrNone
	case 5: // LHU
		return uint32(binary.LittleEndian.Uint16(memory.extram[addr:])), VmErrNone
	}
	return 0, VmErrNone
}

// ExtramStore 根据指定的访问类型（字节、半字、字）将数据写入扩展内存。
// 它处理地址转换和边界检查，并在写入后设置 `extramDirty` 标志。
func (memory *Memory) ExtramStore(addr, val, accessTyp uint32) VmErr {
	if memory.extram == nil {
		return VmErrNone
	}
	addr -= VmEetRamBase
	if addr >= memory.extramLen {
		return VmErrMemWr
	}
	switch accessTyp {
	case 0: // SB
		memory.extram[addr] = byte(val)
	case 1: // SH
		binary.LittleEndian.PutUint16(memory.extram[addr:], uint16(val))
	case 2: // SW
		binary.LittleEndian.PutUint32(memory.extram[addr:], val)
	}
	memory.extramDirty = true
	return VmErrNone
}
