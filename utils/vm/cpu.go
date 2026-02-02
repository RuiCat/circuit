package vm

// VmInaState 定义了 RISC-V 虚拟机核心的状态。
// 这个结构体是 CPU 状态的快照，包含了所有处理器寄存器。
type VmInaState struct {
	// --- 通用寄存器 ---
	Regs [32]uint32 // 32个32位整数通用寄存器 (x0-x31)。
	// --- 浮点寄存器 (F/D-extension) ---
	FRegs [32]uint64 // 32个64位浮点寄存器 (f0-f31)，支持单精度 (F) 和双精度 (D)。
	// --- 向量寄存器 (V-extension) ---
	// 在此实现中，VLEN=128位。总共有32个128位的向量寄存器(v0-v31)，
	// 因此总大小为 32 * 16 = 512 字节。
	Vregs [512]byte

	// --- 核心程序状态 ---
	PC        uint32 // 程序计数器，指向下一条待执行指令的地址。
	Privilege uint8  // 当前的特权级别 (0=User, 1=Supervisor, 3=Machine)。

	// --- 机器模式控制与状态寄存器 (CSRs) ---
	Mstatus  uint32       // 机器状态寄存器，包含全局中断使能和处理器模式等信息。
	Mscratch uint32       // 机器模式下的一个暂存寄存器，供陷阱处理程序使用。
	Mtvec    uint32       // 机器模式陷阱向量基地址，指向陷阱处理程序的入口。
	Mideleg  uint32       // 机器中断委托寄存器。
	Medeleg  uint32       // 机器异常委托寄存器。
	Mie      uint32       // 机器中断使能寄存器，控制哪些中断可以被触发。
	Mip      uint32       // 机器中断挂起寄存器，显示哪些中断正在等待处理。
	Mepc     uint32       // 机器异常程序计数器，保存发生异常或中断时的指令地址。
	Mtval    uint32       // 机器陷阱值寄存器，提供有关陷阱的额外信息（如无效地址或非法指令编码）。
	Mcause   VmMcauseCode // 机器陷阱原因寄存器，指示发生陷阱的具体原因。
	Misa     uint32       // MISA 寄存器，报告支持的指令集体系结构。

	// --- 监控模式控制与状态寄存器 (CSRs) ---
	Sstatus  uint32 // 监控模式状态寄存器。
	Sie      uint32 // 监控模式中断使能。
	Stvec    uint32 // 监控模式陷阱处理程序基地址。
	Sscratch uint32 // 供监控模式陷阱处理程序使用的暂存寄存器。
	Sepc     uint32 // 监控模式异常程序计数器。
	Scause   uint32 // 监控模式陷阱原因。
	Stval    uint32 // 监控模式陷阱值。
	Sip      uint32 // 监控模式中断挂起。
	Satp     uint32 // 监控模式地址翻译与保护。

	// --- 浮点控制与状态寄存器 (F-extension CSR) ---
	Fcsr uint32 // 浮点控制与状态寄存器，包含舍入模式(frm)和异常标志(fflags)。

	// --- 向量扩展控制与状态寄存器 (V-extension CSRs) ---
	Vstart uint32 // 向量起始索引，用于可恢复的向量指令。
	Vl     uint32 // 向量长度寄存器，由 vsetvl(i) 指令设置，表示当前向量操作要处理的元素数。
	Vtype  uint32 // 向量类型寄存器，配置向量元素的位宽(SEW)和寄存器分组(LMUL)。

	Extraflags uint32 // 用于虚拟机特定目的的额外标志位。

	// --- 其他机器模式 CSRs ---
	Mcountinhibit uint32 // CSR 0x3b0: 机器模式计数器禁止寄存器
	Mcycle        uint64 // CSR 0xb00, 0xb80: 机器模式周期计数器 (64位)
	Minstret      uint64 // CSR 0xb02, 0xb82: 机器模式指令执行计数器 (64位)

	// --- A-扩展 (原子指令) ---
	LoadReservation uint32 // 为 LR/SC 指令保留的地址。
}

// CsrRead 从指定的控制和状态寄存器（CSR）中读取值。
//
// 参数:
//
//	csr: 要读取的CSR的地址。
//
// 返回:
//
//	uint32: 读取到的CSR的值。
//	bool:   如果CSR地址有效且可读，则为 true；否则为 false。
func (vmst *VmState) CsrRead(csr uint32) (uint32, bool) {
	switch csr {
	// --- Supervisor CSRs ---
	case CSR_SSTATUS:
		return vmst.Core.Sstatus, true
	case CSR_SIE:
		return vmst.Core.Sie, true
	case CSR_STVEC:
		return vmst.Core.Stvec, true
	case CSR_SSCRATCH:
		return vmst.Core.Sscratch, true
	case CSR_SEPC:
		return vmst.Core.Sepc, true
	case CSR_SCAUSE:
		return vmst.Core.Scause, true
	case CSR_STVAL:
		return vmst.Core.Stval, true
	case CSR_SIP:
		return vmst.Core.Sip, true
	case CSR_SATP:
		return vmst.Core.Satp, true
	// --- Machine CSRs ---
	case CSR_MSTATUS:
		return vmst.Core.Mstatus, true
	case CSR_MISA:
		return vmst.Core.Misa, true
	case CSR_MEDELEG:
		return vmst.Core.Medeleg, true
	case CSR_MIDELEG:
		return vmst.Core.Mideleg, true
	case CSR_MIE:
		return vmst.Core.Mie, true
	case CSR_MTVEC:
		return vmst.Core.Mtvec, true
	case CSR_MSCRATCH:
		return vmst.Core.Mscratch, true
	case CSR_MEPC:
		return vmst.Core.Mepc, true
	case CSR_MCAUSE:
		return uint32(vmst.Core.Mcause), true
	case CSR_MTVAL:
		return vmst.Core.Mtval, true
	case CSR_MIP:
		return vmst.Core.Mip, true
	case CSR_MHARTID:
		// mhartid 是只读寄存器，返回当前硬件线程的 ID
		// 对于单核系统，通常返回 0
		return 0, true
	case CSR_MVENDORID:
		return 0, true // 标准规定，未实现时返回0。
	case CSR_MARCHID:
		return 0, true // 架构ID，未实现时返回0
	case CSR_MIMPID:
		return 0, true // 实现ID，未实现时返回0
	case CSR_MCOUNTINHIBIT: // mcountinhibit - 机器模式计数器禁止寄存器
		// 这是一个可读写的 CSR，用于控制性能计数器
		// 位 0: 禁止 cycle 计数器
		// 位 1: 禁止 time 计数器
		// 位 2: 禁止 instret 计数器
		// 其他位: 保留
		// 默认值为 0，表示所有计数器都启用
		return vmst.Core.Mcountinhibit, true

	// --- 性能计数器 CSRs ---
	case CSR_MCYCLE: // mcycle - 机器模式周期计数器 (低32位)
		return uint32(vmst.Core.Mcycle), true
	case CSR_MCYCLEH: // mcycleh - 机器模式周期计数器 (高32位)
		return uint32(vmst.Core.Mcycle >> 32), true
	case CSR_MINSTRET: // minstret - 机器模式指令执行计数器 (低32位)
		return uint32(vmst.Core.Minstret), true
	case CSR_MINSTRETH: // minstreth - 机器模式指令执行计数器 (高32位)
		return uint32(vmst.Core.Minstret >> 32), true
	case CSR_MCYCLEH_ALIAS: // mcycleh 的别名 (RV32)
		return uint32(vmst.Core.Mcycle >> 32), true
	case CSR_MINSTRETH_ALIAS: // minstreth 的别名 (RV32)
		return uint32(vmst.Core.Minstret >> 32), true

	// --- Floating-Point CSRs ---
	case CSR_FFLAGS:
		// fflags 是 fcsr 的低5位 [4:0]。
		return vmst.Core.Fcsr & 0x1f, true
	case CSR_FRM:
		// frm 是 fcsr 的 [7:5] 位。
		return (vmst.Core.Fcsr >> 5) & 0x7, true
	case CSR_FCSR:
		return vmst.Core.Fcsr, true

	// --- Vector CSRs ---
	case CSR_VSTART:
		return vmst.Core.Vstart, true
	case CSR_VL:
		return vmst.Core.Vl, true
	case CSR_VTYPE:
		return vmst.Core.Vtype, true
	case CSR_VLENB:
		// 在此实现中，VLEN 固定为128位。VLENB 返回以字节为单位的长度。
		return 16, true
	// 机器模式 PMP 寄存器 (0x3a0 - 0x3ef)
	case 0x3a1, 0x3a3, 0x3b1, 0x3b2, 0x3b3, 0x3b4, 0x3b5:
		return 0, true // 假装支持，不拦截执行
	default:
		// 对于任何其他未实现的CSR，读取失败。
		return 0, false
	}
}

// CsrWrite 向指定的控制和状态寄存器（CSR）写入值。
//
// 参数:
//
//	csr:   要写入的CSR的地址。
//	value: 要写入的32位值。
//
// 返回:
//
//	bool: 如果CSR地址有效且可写，则为 true；否则为 false。
func (vmst *VmState) CsrWrite(csr uint32, value uint32) bool {
	switch csr {
	// --- Supervisor CSRs ---
	case CSR_SSTATUS:
		vmst.Core.Sstatus = value
	case CSR_SIE:
		vmst.Core.Sie = value
	case CSR_STVEC:
		vmst.Core.Stvec = value
	case CSR_SSCRATCH:
		vmst.Core.Sscratch = value
	case CSR_SEPC:
		vmst.Core.Sepc = value
	case CSR_SCAUSE:
		vmst.Core.Scause = value
	case CSR_STVAL:
		vmst.Core.Stval = value
	case CSR_SIP:
		vmst.Core.Sip = value
	case CSR_SATP:
		vmst.Core.Satp = value
	// --- Machine CSRs ---
	case CSR_MSTATUS:
		vmst.Core.Mstatus = value
	case CSR_MISA:
		// MISA 寄存器是只读的，写入操作被忽略。
	case CSR_MEDELEG:
		vmst.Core.Medeleg = value
	case CSR_MIDELEG:
		vmst.Core.Mideleg = value
	case CSR_MIE:
		vmst.Core.Mie = value
	case CSR_MTVEC:
		vmst.Core.Mtvec = value
	case CSR_MSCRATCH:
		vmst.Core.Mscratch = value
	case CSR_MEPC:
		vmst.Core.Mepc = value
	case CSR_MCAUSE:
		vmst.Core.Mcause = VmMcauseCode(value)
	case CSR_MTVAL:
		vmst.Core.Mtval = value
	case CSR_MIP:
		vmst.Core.Mip = value
	case CSR_MHARTID:
		// mhartid 是只读寄存器，写入操作被忽略
		// 根据 RISC-V 规范，尝试写入只读 CSR 不会产生异常
	case CSR_MVENDORID:
		// MVENDORID 也是只读的，写入操作被忽略
	case CSR_MARCHID:
		// marchid 是只读寄存器，写入操作被忽略
	case CSR_MIMPID:
		// mimpid 是只读寄存器，写入操作被忽略
	case CSR_MCOUNTINHIBIT: // mcountinhibit - 机器模式计数器禁止寄存器
		// 这是一个可读写的 CSR，用于控制性能计数器
		// 位 0: 禁止 cycle 计数器
		// 位 1: 禁止 time 计数器
		// 位 2: 禁止 instret 计数器
		// 其他位: 保留
		// 只保留有效的位（低3位）
		vmst.Core.Mcountinhibit = value & 0x7

	// --- 性能计数器 CSRs ---
	case CSR_MCYCLE: // mcycle - 机器模式周期计数器 (低32位)
		vmst.Core.Mcycle = (vmst.Core.Mcycle & 0xffffffff00000000) | uint64(value)
	case CSR_MCYCLEH: // mcycleh - 机器模式周期计数器 (高32位)
		vmst.Core.Mcycle = (vmst.Core.Mcycle & 0xffffffff) | (uint64(value) << 32)
	case CSR_MINSTRET: // minstret - 机器模式指令执行计数器 (低32位)
		vmst.Core.Minstret = (vmst.Core.Minstret & 0xffffffff00000000) | uint64(value)
	case CSR_MINSTRETH: // minstreth - 机器模式指令执行计数器 (高32位)
		vmst.Core.Minstret = (vmst.Core.Minstret & 0xffffffff) | (uint64(value) << 32)
	case CSR_MCYCLEH_ALIAS: // mcycleh 的别名 (RV32)
		vmst.Core.Mcycle = (vmst.Core.Mcycle & 0xffffffff) | (uint64(value) << 32)
	case CSR_MINSTRETH_ALIAS: // minstreth 的别名 (RV32)
		vmst.Core.Minstret = (vmst.Core.Minstret & 0xffffffff) | (uint64(value) << 32)

	// --- Floating-Point CSRs ---
	case CSR_FFLAGS:
		// 只更新 fcsr 的低5位。
		vmst.Core.Fcsr = (vmst.Core.Fcsr &^ 0x1f) | (value & 0x1f)
	case CSR_FRM:
		// 只更新 fcsr 的 [7:5] 位。
		vmst.Core.Fcsr = (vmst.Core.Fcsr &^ 0xe0) | ((value & 0x7) << 5)
	case CSR_FCSR:
		vmst.Core.Fcsr = value

	// --- Vector CSRs ---
	case CSR_VSTART:
		vmst.Core.Vstart = value
	case CSR_VL:
		vmst.Core.Vl = value
	case CSR_VTYPE:
		vmst.Core.Vtype = value
	case CSR_VLENB:
		// 在此实现中，VLENB 是只读的。
		// 机器模式 PMP 寄存器 (0x3a0 - 0x3ef)
	case 0x3a1, 0x3a3, 0x3b1, 0x3b2, 0x3b3, 0x3b4, 0x3b5:
		return true // 假装支持，不拦截执行
	default:
		// 对于任何其他未实现的或只读的CSR，写入失败。
		return false
	}
	return true
}

// TranslateAddress 执行 Sv32 虚拟地址到物理地址的翻译。
func (vmst *VmState) TranslateAddress(vaddr uint32, accessType int) (uint32, VmMcauseCode) {
	mode := (vmst.Core.Satp >> 31) & 1
	if mode == 0 {
		return vaddr, CAUSE_TRAP_CODE_OK
	}

	vpn1 := (vaddr >> 22) & 0x3FF
	vpn0 := (vaddr >> 12) & 0x3FF
	offset := vaddr & 0xFFF

	ptbr := (vmst.Core.Satp & 0x3FFFFF) * 4096
	pte1_addr := ptbr + vpn1*4
	pte1, ok := vmst.LoadUint32(pte1_addr)
	if !ok {
		return 0, pageFault(accessType)
	}

	if (pte1 & 1) == 0 {
		return 0, pageFault(accessType)
	}
	if (pte1 & 0b1110) != 0 {
		if (pte1>>10)&0x3FF != 0 {
			return 0, pageFault(accessType)
		}
		if !checkPermissions(pte1, accessType) {
			return 0, pageFault(accessType)
		}
		ppn1 := (pte1 >> 20) & 0xFFF
		paddr := (ppn1 << 22) | (vaddr & 0x3FFFFF)
		return paddr, CAUSE_TRAP_CODE_OK
	}

	ppn0 := (pte1 >> 10) & 0x3FFFFF
	pte0_addr := (ppn0 * 4096) + vpn0*4

	// 【修改】：删除 pte0_addr >= vmst.VmMemorySize 检查
	pte0, ok := vmst.LoadUint32(pte0_addr)
	if !ok {
		return 0, pageFault(accessType)
	}

	if (pte0 & 1) == 0 {
		return 0, pageFault(accessType)
	}
	if !checkPermissions(pte0, accessType) {
		return 0, pageFault(accessType)
	}

	final_paddr := ((pte0>>10)&0x3FFFFF)*4096 + offset
	// 【修改】：删除 final_paddr >= vmst.VmMemorySize 检查
	return final_paddr, CAUSE_TRAP_CODE_OK
}

// pageFault 根据访问类型返回相应的页错误代码。
func pageFault(accessType int) VmMcauseCode {
	switch accessType {
	case VmMemAccessInstruction:
		return CAUSE_INSTRUCTION_PAGE_FAULT
	case VmMemAccessLoad:
		return CAUSE_LOAD_PAGE_FAULT
	case VmMemAccessStore:
		return CAUSE_STORE_PAGE_FAULT
	}
	return CAUSE_ILLEGAL_INSTRUCTION // 不应该发生
}

// checkPermissions 检查页表项是否允许该访问类型。
func checkPermissions(pte uint32, accessType int) bool {
	r := (pte >> 1) & 1
	w := (pte >> 2) & 1
	x := (pte >> 3) & 1

	switch accessType {
	case VmMemAccessInstruction:
		return x == 1
	case VmMemAccessLoad:
		return r == 1
	case VmMemAccessStore:
		return w == 1
	}
	return false
}

// handleTrap 管理 CPU 对异常和中断的响应。
func (vmst *VmState) handleTrap(trap_code VmMcauseCode, trap_val uint32) {
	cause := uint32(trap_code & 0x7FFFFFFF) // 移除符号位以获取纯粹的原因码
	// --- 1. 陷阱委托 ---
	// 决定陷阱应该在哪个特权级别处理（M-mode 或 S-mode）。
	var target_priv uint8
	var deleg_reg uint32
	deleg_reg = vmst.Core.Medeleg // 异常委托
	// 检查相应的原因位是否在委托寄存器中被设置
	if vmst.Core.Privilege < PRIV_MACHINE && (deleg_reg&(1<<cause)) != 0 {
		target_priv = PRIV_SUPERVISOR
	} else {
		target_priv = PRIV_MACHINE
	}

	// --- 2. 保存状态并更新CSRs ---
	switch target_priv {
	case PRIV_MACHINE:
		// 在 M-mode 处理
		vmst.Core.Mepc = vmst.Core.PC
		vmst.Core.Mcause = trap_code
		vmst.Core.Mtval = trap_val

		// 更新 MSTATUS 寄存器
		// MPIE (bit 7) = MIE (bit 3)
		// MIE (bit 3) = 0 (禁用中断)
		// MPP (bits 12:11) = 当前特权级别
		mstatus := vmst.Core.Mstatus
		mpie := (mstatus >> 3) & 1
		mstatus = (mstatus & ^uint32(MSTATUS_MIE)) | (mpie << 7)
		mstatus = (mstatus & ^uint32(MSTATUS_MPP)) | (uint32(vmst.Core.Privilege) << 11)
		vmst.Core.Mstatus = mstatus

		// 跳转到 M-mode 陷阱处理程序
		vmst.Core.Privilege = PRIV_MACHINE
		vmst.Core.PC = vmst.Core.Mtvec

	case PRIV_SUPERVISOR:
		// 在 S-mode 处理
		vmst.Core.Sepc = vmst.Core.PC
		vmst.Core.Scause = uint32(trap_code)
		vmst.Core.Stval = trap_val

		// 更新 SSTATUS 寄存器
		// SPIE (bit 5) = SIE (bit 1)
		// SIE (bit 1) = 0
		// SPP (bit 8) = 当前特权级别
		sstatus := vmst.Core.Sstatus
		spie := (sstatus >> 1) & 1
		sstatus = (sstatus & ^uint32(SSTATUS_SIE)) | (spie << 5)
		sstatus = (sstatus & ^uint32(SSTATUS_SPP)) | (uint32(vmst.Core.Privilege) << 8)
		vmst.Core.Sstatus = sstatus

		// 跳转到 S-mode 陷阱处理程序
		vmst.Core.Privilege = PRIV_SUPERVISOR
		vmst.Core.PC = vmst.Core.Stvec
	}
}

// VmImaStep 是虚拟机的核心执行循环。它负责获取、解码和执行指令。
//
// 参数:
//
//	count: 要执行的最大指令数。
//
// 返回:
//
//	int32: 如果发生陷阱或异常，则返回相应的陷阱代码；如果正常停止（例如通过 ecall 退出），
//	       则返回 TRAP_CODE_EXIT；否则在执行完指定数量的指令后返回0。
func (vmst *VmState) VmImaStep(count int) VmMcauseCode {
	for i := 0; i < count; i++ {
		pc := vmst.Core.PC
		// --- 1. 指令获取 ---
		// TranslateAddress 负责将虚拟地址转为物理地址 (Bare模式下返回原地址)
		paddr, fetch_trap := vmst.TranslateAddress(pc, VmMemAccessInstruction)
		if fetch_trap != 0 {
			vmst.handleTrap(fetch_trap, pc)
			return fetch_trap
		}

		// 所有指令必须至少2字节对齐
		if paddr&1 != 0 {
			vmst.handleTrap(CAUSE_INSTRUCTION_ADDRESS_MISALIGNED, pc)
			return CAUSE_INSTRUCTION_ADDRESS_MISALIGNED
		}

		ir16, ok := vmst.LoadUint16(paddr)
		if !ok {
			vmst.handleTrap(CAUSE_INSTRUCTION_ACCESS_FAULT, pc)
			return CAUSE_INSTRUCTION_ACCESS_FAULT
		}

		var rdid, rval, newPC uint32
		var trap VmMcauseCode
		var ir uint32

		// --- 2. 解码与执行 ---
		if (ir16 & 0x3) != 0x3 {
			// 16位压缩指令
			ir = uint32(ir16)
			rdid, rval, newPC, trap = handleCompressed(vmst, ir16, pc)
		} else {
			// 32位指令
			// 【修复】：使用 LoadUint32，它会自动处理偏移并检查边界
			ir, ok = vmst.LoadUint32(paddr)
			if !ok {
				vmst.handleTrap(CAUSE_INSTRUCTION_ACCESS_FAULT, pc)
				return CAUSE_INSTRUCTION_ACCESS_FAULT
			}
			opcode := ir & 0x7f
			handler, ok := Instructions[opcode]
			if ok {
				rdid, rval, newPC, trap = handler(vmst, ir, pc)
			} else {
				rdid, rval, newPC, trap = handleIllegal(vmst, ir, pc)
			}
		}

		// --- 3. 陷阱处理 ---
		if trap != 0 {
			if trap == CAUSE_TRAP_CODE_OK {
				return trap
			}
			trap_val := ir
			if (ir16 & 0x3) != 0x3 {
				trap_val = uint32(ir16)
			}
			// Load/Store 故障应该提供故障地址 (已经在处理程序中存入 Mtval)
			if trap == CAUSE_LOAD_ACCESS_FAULT || trap == CAUSE_STORE_ACCESS_FAULT || trap == CAUSE_LOAD_ADDRESS_MISALIGNED || trap == CAUSE_STORE_ADDRESS_MISALIGNED {
				switch vmst.Core.Privilege {
				case PRIV_MACHINE:
					trap_val = vmst.Core.Mtval
				case PRIV_SUPERVISOR:
					trap_val = vmst.Core.Stval
				default:
					trap_val = vmst.Core.Mtval
				}
			}
			vmst.handleTrap(trap, trap_val)
			return trap
		}

		// --- 4. 更新计数器 ---
		if (vmst.Core.Mcountinhibit & 0x1) == 0 {
			vmst.Core.Mcycle++
		}
		if (vmst.Core.Mcountinhibit & 0x4) == 0 {
			vmst.Core.Minstret++
		}

		// --- 5. 写回 ---
		if rdid != 0 {
			vmst.Core.Regs[rdid] = rval
		}

		// --- 6. 更新PC ---
		vmst.Core.PC = newPC
	}
	return CAUSE_TRAP_CODE_OK
}

// GetVelementAddr 计算向量寄存器文件中某个逻辑元素的确切字节地址。
// 这个辅助函数是实现向量指令的关键，它处理了 SEW (标准元素宽度) 和 LMUL (向量长度乘数)
// 带来的复杂地址计算。
//
// 参数:
//
//	reg_start_idx: 向量指令中指定的起始向量寄存器索引 (如 v0, v8 等)。
//	element_idx:   要访问的元素的逻辑索引 (范围从 0 到 vl-1)。
//	sew_bytes:     当前配置的SEW（每个元素的大小），以字节为单位。
//
// 返回:
//
//	uint32: 该元素在 `vmst.Core.Vregs` 字节数组中的绝对字节偏移量。
func (vmst *VmState) GetVelementAddr(reg_start_idx uint32, element_idx uint32, sew_bytes uint32) uint32 {
	const VLEN_BYTES = 16 // VLEN (向量寄存器的物理大小) 在此实现中固定为128位（16字节）。

	// 计算一个128位的物理向量寄存器可以容纳多少个当前SEW的元素。
	elements_per_reg := VLEN_BYTES / sew_bytes

	// `reg_offset` 确定了逻辑元素 `element_idx` 相对于起始物理寄存器 `reg_start_idx`
	// 偏移了多少个物理寄存器。
	// 例如，如果每个物理寄存器能放4个元素，那么第5个逻辑元素(element_idx=4)就在第2个物理寄存器中(reg_offset=1)。
	reg_offset := element_idx / elements_per_reg
	// `element_offset_in_reg` 计算出该元素在它所在的那个物理寄存器内部的字节偏移量。
	element_offset_in_reg := (element_idx % elements_per_reg) * sew_bytes

	// `actual_reg_idx` 是该元素所在的物理向量寄存器的绝对索引。
	actual_reg_idx := reg_start_idx + reg_offset
	// `addr` 是最终的字节地址，即在整个 `Vregs` 数组中的偏移量。
	addr := actual_reg_idx*VLEN_BYTES + element_offset_in_reg

	return addr
}
