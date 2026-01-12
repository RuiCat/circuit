package vm

import (
	"encoding/binary"
)

// VmErr 定义了虚拟机可能出现的错误类型
type VmErr int

// VmEvtTyp 定义了虚拟机事件类型
type VmEvtTyp int

// VmEvtErr 描述了一个错误事件的详情
type VmEvtErr struct {
	Errcode VmErr
	Errstr  string
}

// VmEvtSyscall 描述了一个系统调用事件的详情
type VmEvtSyscall struct {
	Code   uint32
	Ret    *uint32
	Params [2]*uint32
}

// VmEvt 描述了一个虚拟机事件
type VmEvt struct {
	Typ     VmEvtTyp
	Syscall VmEvtSyscall
	Err     VmEvtErr
}

// VmStatus 定义了虚拟机的状态
type VmStatus int

// VmSlice 表示 VM 中的一块内存
type VmSlice struct {
	Ptr []byte
	Len uint32
}

// VmArg 定义了系统调用参数的标识符
type VmArg int

// VmState 表示虚拟机的完整状态
type VmState struct {
	Status      VmStatus
	Err         VmErr
	Core        VmInaState
	Memory      [VmMemoRySize]byte
	Ioevt       VmEvt
	StackCanary *byte
	Garbage     uint32
	extram      []byte
	extramLen   uint32
	extramDirty bool
}

// NewVmState 创建一个新的 VmState
func NewVmState() *VmState {
	vmst := &VmState{}
	vmst.Core.PC = VmRamImageOffSet
	// 设置栈指针，16字节对齐
	vmst.Core.Regs[2] = ((VmRamImageOffSet + VmMemoRySize) &^ 0xF) - 16
	vmst.Core.Extraflags |= 3 // 机器模式
	return vmst
}

// Load 将 ROM 加载到虚拟机内存中
func (vmst *VmState) Load(rom []byte) bool {
	if len(rom) > VmMemoRySize {
		return false
	}
	copy(vmst.Memory[:], rom)
	vmst.StackCanary = nil
	return true
}

// setStatus 设置虚拟机的状态
func (vmst *VmState) setStatus(newStatus VmStatus) {
	if vmst.Status != VmStatusError {
		vmst.Status = newStatus
	}
}

// setStatusErr 设置虚拟机的错误状态
func (vmst *VmState) setStatusErr(err VmErr) {
	if vmst.Status != VmStatusError {
		vmst.setStatus(VmStatusError)
		vmst.Err = err
	}
}

// Run 运行虚拟机，执行 instr_meter 条指令
func (vmst *VmState) Run(instr_meter uint32) (uint32, VmEvt) {
	var evt VmEvt
	orig_instr_meter := instr_meter
	if instr_meter < 1 {
		instr_meter = 1
		orig_instr_meter = 1
	}

	vmst.extramDirty = false

	if vmst.Status != VmStatusPaused {
		vmst.setStatusErr(VmErrNotrEady)
		evt.Typ = VmEvtTypErr
		evt.Err.Errcode = vmst.Err
		return 0, evt
	}

	vmst.setStatus(VmStatusRunnIng)

	for vmst.Status == VmStatusRunnIng && instr_meter > 0 {
		ret := vmst.VmImaStep(1)
		instr_meter--

		switch ret {
		case 0: // OK
		case 12: // ECALL
			syscall := vmst.Core.Regs[17] // a7
			vmst.Core.PC += 4
			switch syscall {
			case VmSysCallHalt:
				vmst.setStatus(VmStatusEnded)
			default:
				vmst.Ioevt.Typ = VmEvtTypSysCall
				vmst.Ioevt.Syscall.Code = syscall
				vmst.Ioevt.Syscall.Ret = &vmst.Core.Regs[12]       // a2
				vmst.Ioevt.Syscall.Params[0] = &vmst.Core.Regs[10] // a0
				vmst.Ioevt.Syscall.Params[1] = &vmst.Core.Regs[11] // a1
				vmst.setStatus(VmStatusPaused)
			}
		case 6: // 加载访问故障
			vmst.setStatusErr(VmErrMemRd)
		default: // 未处理的异常
			vmst.setStatusErr(VmErrIntErnalCore)
		}

		if vmst.Status == VmStatusRunnIng && instr_meter == 0 {
			vmst.setStatusErr(VmErrHung)
		}
	}

	executed_instrs := orig_instr_meter - instr_meter

	if vmst.Status == VmStatusEnded {
		evt.Typ = VmEvtTypEnd
		return executed_instrs, evt
	}

	if vmst.Status == VmStatusPaused {
		return executed_instrs, vmst.Ioevt
	}

	if vmst.Status == VmStatusError {
		evt.Typ = VmEvtTypErr
		evt.Err.Errcode = vmst.Err
	}

	return executed_instrs, evt
}

// HasEnded 检查虚拟机是否已经结束
func (vmst *VmState) HasEnded() bool {
	return vmst.Status == VmStatusEnded
}

// ClearError 清除错误状态
func (vmst *VmState) ClearError() {
	if vmst.Status == VmStatusError {
		vmst.Status = VmStatusPaused
	}
}

// Extram 设置扩展内存
func (vmst *VmState) Extram(ram []byte) {
	vmst.extram = ram
	vmst.extramLen = uint32(len(ram))
}

// ExtramDirty 检查扩展内存是否被修改
func (vmst *VmState) ExtramDirty() bool {
	return vmst.extramDirty
}

// GetMemory 获取主内存
func (vmst *VmState) GetMemory() []byte {
	return vmst.Memory[:]
}

// GetProgramCounter 获取程序计数器
func (vmst *VmState) GetProgramCounter() uint32 {
	return vmst.Core.PC
}

// SetProgramCounter 设置程序计数器
func (vmst *VmState) SetProgramCounter(pc uint32) {
	vmst.Core.PC = pc
}

// GetSafePtr 获取一个安全的内存指针
func (vmst *VmState) GetSafePtr(addr, length uint32) ([]byte, bool) {
	if minirv32_mmio_range(addr) {
		if vmst.extram == nil {
			return nil, false
		}
		ptrstart := addr - VmEetRamBase
		if ptrstart > vmst.extramLen || ptrstart+length > vmst.extramLen {
			vmst.setStatusErr(VmErrMemRd)
			return nil, false
		}
		return vmst.extram[ptrstart : ptrstart+length], true
	}

	ptrstart := addr - VmRamImageOffSet
	if ptrstart > VmMemoRySize || ptrstart+length > VmMemoRySize {
		vmst.setStatusErr(VmErrMemRd)
		return nil, false
	}
	return vmst.Memory[ptrstart : ptrstart+length], true
}

// ArgGetVal 获取系统调用参数的值
func (vmst *VmState) ArgGetVal(evt *VmEvt, arg VmArg) uint32 {
	ptr := vmst.argToPtr(evt, arg)
	if ptr == nil {
		return 0
	}
	return *ptr
}

// ArgSetVal 设置系统调用参数的值
func (vmst *VmState) ArgSetVal(evt *VmEvt, arg VmArg, val uint32) {
	ptr := vmst.argToPtr(evt, arg)
	if ptr != nil {
		*ptr = val
	}
}

// argToPtr 将系统调用参数标识符转换为指针
func (vmst *VmState) argToPtr(evt *VmEvt, arg VmArg) *uint32 {
	switch arg {
	case Arg0:
		return evt.Syscall.Params[0]
	case Arg1:
		return evt.Syscall.Params[1]
	case Ret:
		return evt.Syscall.Ret
	default:
		vmst.setStatusErr(VmErrArgs)
		return &vmst.Garbage
	}
}

// ExtramLoad 从扩展内存加载数据
func (vmst *VmState) ExtramLoad(addr uint32, accessTyp uint32) uint32 {
	if vmst.extram == nil {
		return 0
	}
	addr -= VmEetRamBase
	if addr >= vmst.extramLen {
		vmst.setStatusErr(VmErrMemRd)
		return 0
	}

	switch accessTyp {
	case 0: // LB
		return uint32(int8(vmst.extram[addr]))
	case 1: // LH
		return uint32(int16(binary.LittleEndian.Uint16(vmst.extram[addr:])))
	case 2: // LW
		return binary.LittleEndian.Uint32(vmst.extram[addr:])
	case 4: // LBU
		return uint32(vmst.extram[addr])
	case 5: // LHU
		return uint32(binary.LittleEndian.Uint16(vmst.extram[addr:]))
	}
	return 0
}

// extramStore 将数据存储到扩展内存
func (vmst *VmState) extramStore(addr, val, accessTyp uint32) {
	if vmst.extram == nil {
		return
	}
	addr -= VmEetRamBase
	if addr >= vmst.extramLen {
		vmst.setStatusErr(VmErrMemWr)
		return
	}

	switch accessTyp {
	case 0: // SB
		vmst.extram[addr] = byte(val)
	case 1: // SH
		binary.LittleEndian.PutUint16(vmst.extram[addr:], uint16(val))
	case 2: // SW
		binary.LittleEndian.PutUint32(vmst.extram[addr:], val)
	}
	vmst.extramDirty = true
}

// minirv32_mmio_range 检查地址是否在 MMIO 范围内
func minirv32_mmio_range(n uint32) bool {
	return VmEetRamBase <= n && n < 0x12000000
}
