package vm

import (
	"encoding/binary"
)

// Device 接口定义了内存映射设备的基本操作
type Device interface {
	// Read 从设备读取数据
	Read(addr uint32, size uint32) (uint32, bool)
	// Write 向设备写入数据
	Write(addr uint32, value uint32, size uint32) bool
	// GetName 返回设备名称
	GetName() string
	// GetBaseAddr 返回设备基地址
	GetBaseAddr() uint32
	// GetSize 返回设备大小
	GetSize() uint32
	// Tick 中断处理
	Tick(vmst *VmState)
}

// DeviceManager 管理所有设备
type DeviceManager struct {
	devices []Device
}

// NewDeviceManager 创建新的设备管理器
func NewDeviceManager() *DeviceManager {
	return &DeviceManager{
		devices: make([]Device, 0),
	}
}

// AddDevice 添加设备
func (dm *DeviceManager) AddDevice(dev Device) {
	dm.devices = append(dm.devices, dev)
}

// FindDevice 根据地址查找设备
func (dm *DeviceManager) FindDevice(addr uint32) (dev Device, _ uint32) {
	for i := range dm.devices {
		dev = dm.devices[i]
		base := dev.GetBaseAddr()
		size := dev.GetSize()
		if addr >= base && addr < base+size {
			return dev, addr - base
		}
	}
	return nil, 0
}

// FindTick 周期调用
func (dm *DeviceManager) FindTick(vmst *VmState) {
	for i := range dm.devices {
		dm.devices[i].Tick(vmst)
	}
}

// Memory 内存封装
type Memory struct {
	Dm             *DeviceManager // 设备管理器
	RamImageOffSet uint32         // 内存起始位置
	Data           []byte         // 内存数据
	VmMemorySize   uint32         // 主内存大小
}

// Load 加载内存
func (memory *Memory) Load(rom []byte) {
	copy(memory.Data[:], rom)
}

// LoadUint8 读取8位数据，支持设备
func (memory *Memory) LoadUint8(addr uint32) (uint8, bool) {
	// 检查是否是设备地址
	if memory.Dm != nil {
		if dev, offset := memory.Dm.FindDevice(addr); dev != nil {
			value, ok := dev.Read(offset, 1)
			if ok {
				return uint8(value & 0xFF), true
			}
			return 0, false
		}
	}
	// 主内存访问，需要检查边界
	// 首先检查地址是否在内存范围内
	if addr >= memory.RamImageOffSet && addr < memory.RamImageOffSet+memory.VmMemorySize {
		// 计算内存数组中的偏移量
		offset := addr - memory.RamImageOffSet
		return memory.Data[offset], true
	}
	return 0, false
}

// LoadUint16 读取16位数据，支持设备
func (memory *Memory) LoadUint16(addr uint32) (uint16, bool) {
	// 检查是否是设备地址
	if memory.Dm != nil {
		if dev, offset := memory.Dm.FindDevice(addr); dev != nil {
			value, ok := dev.Read(offset, 2)
			if ok {
				return uint16(value & 0xFFFF), true
			}

			return 0, false
		}
	}
	// 主内存访问，需要检查边界
	// 首先检查地址是否在内存范围内
	if addr >= memory.RamImageOffSet && addr+1 < memory.RamImageOffSet+memory.VmMemorySize {
		// 计算内存数组中的偏移量
		offset := addr - memory.RamImageOffSet
		return binary.LittleEndian.Uint16(memory.Data[offset:]), true
	}
	return 0, false
}

// LoadUint32 读取32位数据，支持设备
func (memory *Memory) LoadUint32(addr uint32) (uint32, bool) {
	// 检查是否是设备地址
	if memory.Dm != nil {
		if dev, offset := memory.Dm.FindDevice(addr); dev != nil {
			value, ok := dev.Read(offset, 4)
			if ok {
				return value, true
			}
			return 0, false
		}
	}
	// 主内存访问，需要检查边界
	// 首先检查地址是否在内存范围内
	if addr >= memory.RamImageOffSet && addr+3 < memory.RamImageOffSet+memory.VmMemorySize {
		// 计算内存数组中的偏移量
		offset := addr - memory.RamImageOffSet
		return binary.LittleEndian.Uint32(memory.Data[offset:]), true
	}
	return 0, false
}

// LoadUint64 读取64位数据，支持设备
func (memory *Memory) LoadUint64(addr uint32) (uint64, bool) {
	// 检查是否是设备地址
	if memory.Dm != nil {
		if dev, offset := memory.Dm.FindDevice(addr); dev != nil {
			// 设备接口只支持32位读取，所以需要读取两次
			// 先读取低32位
			low, ok := dev.Read(offset, 4)
			if !ok {
				return 0, false
			}
			// 再读取高32位
			high, ok := dev.Read(offset+4, 4)
			if !ok {
				return 0, false
			}
			return (uint64(high) << 32) | uint64(low), true
		}
	}
	// 主内存访问，需要检查边界
	// 首先检查地址是否在内存范围内
	if addr >= memory.RamImageOffSet && addr+7 < memory.RamImageOffSet+memory.VmMemorySize {
		// 计算内存数组中的偏移量
		offset := addr - memory.RamImageOffSet
		return binary.LittleEndian.Uint64(memory.Data[offset:]), true
	}
	return 0, false
}

// PutUint8 写入8位数据，支持设备
func (memory *Memory) PutUint8(addr uint32, value uint8) bool {
	// 检查是否是设备地址
	if memory.Dm != nil {
		if dev, offset := memory.Dm.FindDevice(addr); dev != nil {
			return dev.Write(offset, uint32(value), 1)
		}
	}
	// 主内存访问，需要检查边界
	// 首先检查地址是否在内存范围内
	if addr >= memory.RamImageOffSet && addr < memory.RamImageOffSet+memory.VmMemorySize {
		// 计算内存数组中的偏移量
		offset := addr - memory.RamImageOffSet
		memory.Data[offset] = value
		return true
	}
	return false
}

// PutUint16 写入16位数据，支持设备
func (memory *Memory) PutUint16(addr uint32, value uint16) bool {
	// 检查是否是设备地址
	if memory.Dm != nil {
		if dev, offset := memory.Dm.FindDevice(addr); dev != nil {
			return dev.Write(offset, uint32(value), 2)
		}
	}
	// 主内存访问，需要检查边界
	// 首先检查地址是否在内存范围内
	if addr >= memory.RamImageOffSet && addr+1 < memory.RamImageOffSet+memory.VmMemorySize {
		// 计算内存数组中的偏移量
		offset := addr - memory.RamImageOffSet
		binary.LittleEndian.PutUint16(memory.Data[offset:], value)
		return true
	}
	return false
}

// PutUint32 写入32位数据，支持设备
func (memory *Memory) PutUint32(addr uint32, value uint32) bool {
	// 检查是否是设备地址
	if memory.Dm != nil {
		if dev, offset := memory.Dm.FindDevice(addr); dev != nil {
			return dev.Write(offset, value, 4)
		}
	}
	// 主内存访问，需要检查边界
	// 首先检查地址是否在内存范围内
	if addr >= memory.RamImageOffSet && addr+3 < memory.RamImageOffSet+memory.VmMemorySize {
		// 计算内存数组中的偏移量
		offset := addr - memory.RamImageOffSet
		binary.LittleEndian.PutUint32(memory.Data[offset:], value)
		return true
	}
	return false
}

// PutUint64 写入64位数据，支持设备
func (memory *Memory) PutUint64(addr uint32, value uint64) bool {
	// 检查是否是设备地址
	if memory.Dm != nil {
		if dev, offset := memory.Dm.FindDevice(addr); dev != nil {
			// 设备接口只支持32位写入，所以需要写入两次
			// 先写入低32位
			low := uint32(value & 0xFFFFFFFF)
			if !dev.Write(offset, low, 4) {
				return false
			}
			// 再写入高32位
			high := uint32(value >> 32)
			return dev.Write(offset+4, high, 4)
		}
	}
	// 主内存访问，需要检查边界
	// 首先检查地址是否在内存范围内
	if addr >= memory.RamImageOffSet && addr+7 < memory.RamImageOffSet+memory.VmMemorySize {
		// 计算内存数组中的偏移量
		offset := addr - memory.RamImageOffSet
		binary.LittleEndian.PutUint64(memory.Data[offset:], value)
		return true
	}
	return false
}

// GetMemory 返回一个指向虚拟机主内存的字节切片。
func (memory *Memory) GetMemory() []byte {
	return memory.Data[:]
}
