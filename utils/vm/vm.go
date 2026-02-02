package vm

// VmErr 定义了虚拟机可能出现的错误类型
type VmErr int

// VmEvtTyp 定义了虚拟机事件类型
type VmEvtTyp int

// VmEvtErr 描述了一个错误事件的详情。
type VmEvtErr struct {
	Errcode VmErr  // 错误码，用于程序化处理。
	Errstr  string // 错误的字符串描述，用于调试和日志。
}

// VmEvtSyscall 描述了一个系统调用事件的详情。
// 当虚拟机执行 ECALL 指令时，会生成此事件，并暂停执行，等待外部环境处理。
type VmEvtSyscall struct {
	Code   uint32     // 系统调用号（来自 a7 寄存器）。
	Ret    *uint32    // 指向返回值寄存器（a2）的指针，外部环境可以通过此指针写回返回值。
	Params [2]*uint32 // 指向参数寄存器（a0, a1）的指针数组。
}

// VmEvt 描述了一个虚拟机事件，它是虚拟机与外部环境通信的主要方式。
type VmEvt struct {
	Typ     VmEvtTyp     // 事件类型（错误、系统调用、结束等）。
	Syscall VmEvtSyscall // 如果事件是系统调用，则此字段包含详细信息。
	Err     VmEvtErr     // 如果事件是错误，则此字段包含详细信息。
}

// VmStatus 定义了虚拟机的状态
type VmStatus int

// VmSlice 表示虚拟机内存中的一个连续区域，由指针和长度定义。
type VmSlice struct {
	Ptr []byte // 指向内存区域起始位置的指针
	Len uint32 // 内存区域的长度
}

// VmArg 定义了系统调用参数的类型标识符，用于安全地访问参数。
type VmArg int

// VmState 表示虚拟机的完整状态，包括核心、内存、I/O事件和状态标志。
type VmState struct {
	Memory                 // 主内存区域。
	Status      VmStatus   // 虚拟机的当前运行状态 (例如，运行中、暂停、错误)。
	Err         VmErr      // 如果发生错误，记录错误代码。
	Core        VmInaState // 虚拟机核心的状态，包括寄存器和程序计数器。
	Ioevt       VmEvt      // 当前待处理的I/O事件，如系统调用。
	StackCanary *byte      // 栈保护金丝雀值，用于检测栈溢出（当前未使用）。
	Garbage     uint32     // 一个丢弃值的存储位置，用于无效的指针操作。
	lastIR      uint32     // 最近执行的指令，用于某些指令的内部状态。
}

// NewVmState 创建并初始化一个新的虚拟机状态实例。
func NewVmState(VmMemorySize uint32) *VmState {
	vmst := &VmState{
		Memory: Memory{
			Data:           make([]byte, int(VmMemorySize)),
			VmMemorySize:   VmMemorySize,
			RamImageOffSet: 0x80000000, // RAM 在地址空间中的起始偏移。加载的程序镜像将从这里开始。
		},
	}
	// 将程序计数器（PC）初始化为RAM镜像的起始偏移量。
	vmst.Core.PC = vmst.RamImageOffSet
	// 将栈指针（sp, x2）设置在主内存的末尾，并确保16字节对齐。
	vmst.Core.Regs[2] = ((vmst.RamImageOffSet + vmst.VmMemorySize) &^ 0xF) - 16
	// 设置 MISA 寄存器，表明支持 RV32IMAFD 扩展。
	// MXL=1 (RV32), I, M, A, F, D 扩展。
	vmst.Core.Misa = 0x40001129
	// 设置CPU模式为机器模式（Machine Mode）。
	vmst.Core.Extraflags |= 3
	return vmst
}

// Load 将提供的ROM字节切片加载到虚拟机的内存中。
// 如果ROM的大小超过虚拟机内存容量，则加载失败。
func (vmst *VmState) Load(rom []byte) bool {
	if len(rom) > int(vmst.VmMemorySize) {
		return false
	}
	vmst.Load(rom)
	vmst.StackCanary = nil
	return true
}

// SetStatus 安全地设置虚拟机的状态，除非虚拟机已经处于错误状态。
func (vmst *VmState) SetStatus(newStatus VmStatus) {
	if vmst.Status != VmStatusError {
		vmst.Status = newStatus
	}
}

// SetStatusErr 将虚拟机的状态设置为错误，并记录具体的错误代码。
// 此操作是不可逆的，直到错误被显式清除。
func (vmst *VmState) SetStatusErr(err VmErr) {
	if vmst.Status != VmStatusError {
		vmst.SetStatus(VmStatusError)
		vmst.Err = err
	}
}

// Run 运行虚拟机执行指定数量的指令。
// 这是虚拟机的主执行循环，负责处理指令的获取、解码、执行，
// 并管理系统调用、中断和异常。
// 它会持续执行，直到指定的指令数用尽、发生需要暂停的事件（如系统调用）、
// 虚拟机执行完毕或遇到无法恢复的错误。
//
// 参数:
//
//	instr_meter: 本次运行允许执行的最大指令数。
//
// 返回值:
//
//	uint32: 实际执行的指令数。
//	VmEvt:  执行过程中发生的事件。可能是 VmEvtTypEnd（结束）、
//	        VmEvtTypSysCall（系统调用）、或 VmEvtTypErr（错误）。
func (vmst *VmState) Run(instr_meter uint32) (uint32, VmEvt) {
	vmst.ClearError() // 重置错误
	var evt VmEvt
	orig_instr_meter := instr_meter
	if instr_meter < 1 {
		instr_meter = 1
		orig_instr_meter = 1
	}
	if vmst.Status != VmStatusPaused {
		vmst.SetStatusErr(VmErrNotrEady)
		evt.Typ = VmEvtTypErr
		evt.Err.Errcode = vmst.Err
		evt.Err.Errstr = vmst.errToString(vmst.Err)
		return 0, evt
	}
	vmst.SetStatus(VmStatusRunnIng)
	for vmst.Status == VmStatusRunnIng && instr_meter > 0 {
		// 只有当全局中断开启 (MIE) 时才处理
		if (vmst.Core.Mstatus & MSTATUS_MIE) != 0 {
			// 检查哪些中断被使能 (mie) 且 正在挂起 (mip)
			enabled_interrupts := vmst.Core.Mie & vmst.Core.Mip
			if (enabled_interrupts & (1 << 7)) != 0 {
				vmst.handleTrap(CAUSE_MACHINE_TIMER_INTERRUPT, vmst.Core.PC)
			}
		}
		// 执行单条指令，并获取执行结果（陷阱码）。
		ret := vmst.VmImaStep(1)
		// 中断调用
		if vmst.Dm != nil {
			vmst.Dm.FindTick(vmst)
		}
		instr_meter--
		// 处理指令执行结果
		switch ret {
		case CAUSE_TRAP_CODE_OK:
		case CAUSE_INSTRUCTION_PAGE_FAULT,
			CAUSE_LOAD_ADDRESS_MISALIGNED,
			CAUSE_LOAD_ACCESS_FAULT,
			CAUSE_INSTRUCTION_ADDRESS_MISALIGNED,
			CAUSE_INSTRUCTION_ACCESS_FAULT,
			CAUSE_LOAD_PAGE_FAULT:
			vmst.Core.Mcause = ret
			vmst.SetStatusErr(VmErrMemRd)
		case CAUSE_STORE_ACCESS_FAULT,
			CAUSE_STORE_ADDRESS_MISALIGNED,
			CAUSE_STORE_PAGE_FAULT:
			vmst.Core.Mcause = ret
			vmst.SetStatusErr(VmErrMemWr)
		case CAUSE_ILLEGAL_INSTRUCTION:
			vmst.Core.Mcause = ret
			vmst.SetStatusErr(VmErrIntErnalCore)
		case CAUSE_BREAKPOINT,
			CAUSE_USER_ECALL,
			CAUSE_SUPERVISOR_ECALL,
			CAUSE_MACHINE_ECALL:
			// 生成系统调用事件而不是错误
			vmst.Core.Mcause = ret
			vmst.Ioevt.Typ = VmEvtTypSysCall
			vmst.Ioevt.Syscall.Code = vmst.Core.Regs[17]       // a7寄存器包含系统调用号
			vmst.Ioevt.Syscall.Params[0] = &vmst.Core.Regs[10] // a0
			vmst.Ioevt.Syscall.Params[1] = &vmst.Core.Regs[11] // a1
			vmst.Ioevt.Syscall.Ret = &vmst.Core.Regs[10]       // a0作为返回值
			vmst.SetStatus(VmStatusPaused)
			return orig_instr_meter - instr_meter, vmst.Ioevt
		default:
			vmst.Core.Mcause = ret
			vmst.SetStatusErr(VmErrIntErnalCore)
		}
		if vmst.Status == VmStatusRunnIng && instr_meter == 0 {
			vmst.SetStatus(VmStatusPaused)
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
		evt.Err.Errstr = vmst.errToString(vmst.Err)
	}
	return executed_instrs, evt
}

// HasEnded 返回虚拟机是否已执行完毕并进入结束状态。
func (vmst *VmState) HasEnded() bool {
	return vmst.Status == VmStatusEnded
}

// ClearError 如果虚拟机处于错误状态，则将其重置为暂停状态，以允许继续执行或进行调试。
func (vmst *VmState) ClearError() {
	if vmst.Status == VmStatusError {
		vmst.Status = VmStatusPaused
	}
}

// GetProgramCounter 返回程序计数器（PC）的当前值。
func (vmst *VmState) GetProgramCounter() uint32 {
	return vmst.Core.PC
}

// SetProgramCounter 设置程序计数器（PC）的值。
func (vmst *VmState) SetProgramCounter(pc uint32) {
	vmst.Core.PC = pc
}

// ArgGetVal 从给定的虚拟机事件中检索指定系统调用参数的值。
func (vmst *VmState) ArgGetVal(evt *VmEvt, arg VmArg) uint32 {
	ptr := vmst.argToPtr(evt, arg)
	if ptr == nil {
		return 0
	}
	return *ptr
}

// ArgSetVal 为给定虚拟机事件中的指定系统调用参数设置一个新值。
func (vmst *VmState) ArgSetVal(evt *VmEvt, arg VmArg, val uint32) {
	ptr := vmst.argToPtr(evt, arg)
	if ptr != nil {
		*ptr = val
	}
}

// argToPtr 是一个辅助函数，它将系统调用参数标识符（如 Arg0, Arg1, Ret）
// 转换为指向事件中相应值的指针，以便于读写。
func (vmst *VmState) argToPtr(evt *VmEvt, arg VmArg) *uint32 {
	switch arg {
	case Arg0:
		return evt.Syscall.Params[0]
	case Arg1:
		return evt.Syscall.Params[1]
	case Ret:
		return evt.Syscall.Ret
	default:
		vmst.SetStatusErr(VmErrArgs)
		return &vmst.Garbage
	}
}

// errToString 将错误代码转换为可读的字符串描述
func (vmst *VmState) errToString(err VmErr) string {
	switch err {
	case VmErrNone:
		return "无错误"
	case VmErrNotrEady:
		return "虚拟机尚未准备好执行"
	case VmErrMemRd:
		return "内存读取错误"
	case VmErrMemWr:
		return "内存写入错误"
	case VmErrBadSysCall:
		return "无效的系统调用代码"
	case VmErrHung:
		return "虚拟机挂起"
	case VmErrIntErnalCore:
		return "内部核心逻辑错误"
	case VmErrArgs:
		return "传递给VM的参数错误"
	default:
		return "未知错误"
	}
}
