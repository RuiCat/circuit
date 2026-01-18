package vm

import (
	"encoding/binary"
)

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
	PC uint32 // 程序计数器，指向下一条待执行指令的地址。

	// --- 机器模式控制与状态寄存器 (CSRs) ---
	Mstatus  uint32 // 机器状态寄存器，包含全局中断使能和处理器模式等信息。
	Mscratch uint32 // 机器模式下的一个暂存寄存器，供陷阱处理程序使用。
	Mtvec    uint32 // 机器模式陷阱向量基地址，指向陷阱处理程序的入口。
	Mie      uint32 // 机器中断使能寄存器，控制哪些中断可以被触发。
	Mip      uint32 // 机器中断挂起寄存器，显示哪些中断正在等待处理。
	Mepc     uint32 // 机器异常程序计数器，保存发生异常或中断时的指令地址。
	Mtval    uint32 // 机器陷阱值寄存器，提供有关陷阱的额外信息（如无效地址或非法指令编码）。
	Mcause   uint32 // 机器陷阱原因寄存器，指示发生陷阱的具体原因。

	// --- 浮点控制与状态寄存器 (F-extension CSR) ---
	Fcsr uint32 // 浮点控制与状态寄存器，包含舍入模式(frm)和异常标志(fflags)。

	// --- 向量扩展控制与状态寄存器 (V-extension CSRs) ---
	Vstart uint32 // 向量起始索引，用于可恢复的向量指令。
	Vl     uint32 // 向量长度寄存器，由 vsetvl(i) 指令设置，表示当前向量操作要处理的元素数。
	Vtype  uint32 // 向量类型寄存器，配置向量元素的位宽(SEW)和寄存器分组(LMUL)。

	Extraflags uint32 // 用于虚拟机特定目的的额外标志位。

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
	case CSR_MSTATUS:
		return vmst.Core.Mstatus, true
	case CSR_MISA:
		// 硬编码返回一个值，表示支持 RV32IMAFDV 扩展。
		// I, M, A, F, D (双精度) 和 V (向量)
		return 0x40101101, true
	case CSR_MIE:
		return vmst.Core.Mie, true
	case CSR_MTVEC:
		return vmst.Core.Mtvec, true
	case CSR_MSCRATCH:
		return vmst.Core.Mscratch, true
	case CSR_MEPC:
		return vmst.Core.Mepc, true
	case CSR_MCAUSE:
		return vmst.Core.Mcause, true
	case CSR_MTVAL:
		return vmst.Core.Mtval, true
	case CSR_MIP:
		return vmst.Core.Mip, true
	case CSR_MVENDORID:
		return 0, true // 标准规定，未实现时返回0。
	case CSR_FFLAGS:
		// fflags 是 fcsr 的低5位 [4:0]。
		return vmst.Core.Fcsr & 0x1f, true
	case CSR_FRM:
		// frm 是 fcsr 的 [7:5] 位。
		return (vmst.Core.Fcsr >> 5) & 0x7, true
	case CSR_FCSR:
		return vmst.Core.Fcsr, true
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
	case CSR_MSTATUS:
		vmst.Core.Mstatus = value
	case CSR_MISA:
		// MISA 寄存器是只读的，写入操作被忽略。
	case CSR_MIE:
		vmst.Core.Mie = value
	case CSR_MTVEC:
		vmst.Core.Mtvec = value
	case CSR_MSCRATCH:
		vmst.Core.Mscratch = value
	case CSR_MEPC:
		vmst.Core.Mepc = value
	case CSR_MCAUSE:
		vmst.Core.Mcause = value
	case CSR_MTVAL:
		vmst.Core.Mtval = value
	case CSR_MIP:
		vmst.Core.Mip = value
	case CSR_FFLAGS:
		// 只更新 fcsr 的低5位。
		vmst.Core.Fcsr = (vmst.Core.Fcsr &^ 0x1f) | (value & 0x1f)
	case CSR_FRM:
		// 只更新 fcsr 的 [7:5] 位。
		vmst.Core.Fcsr = (vmst.Core.Fcsr &^ 0xe0) | ((value & 0x7) << 5)
	case CSR_FCSR:
		vmst.Core.Fcsr = value
	default:
		// 对于任何其他未实现的或只读的CSR，写入失败。
		return false
	}
	return true
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
func (vmst *VmState) VmImaStep(count int) int32 {
	var trap int32 = 0
	pc := vmst.Core.PC

	for range count {
		// --- 1. 指令获取 (Instruction Fetch) ---
		ofs_pc := pc - VmRamImageOffSet
		// 检查PC是否在有效内存范围内
		if ofs_pc >= VmMemoRySize {
			trap = CAUSE_INSTRUCTION_ACCESS_FAULT
			break
		}
		// 检查PC是否是4字节对齐的
		if ofs_pc&3 != 0 {
			trap = CAUSE_INSTRUCTION_ADDRESS_MISALIGNED
			break
		}

		// 从内存中以小端模式读取32位指令
		ir := binary.LittleEndian.Uint32(vmst.Memory[ofs_pc:])
		opcode := ir & 0x7f

		var rdid, rval, newPC uint32

		// --- 2. 指令解码与执行 (Decode & Execute) ---
		// 通过操作码在指令映射表中查找对应的处理函数
		handler, ok := Instructions[opcode]
		if ok {
			// 如果找到，则调用处理函数
			rdid, rval, newPC, trap = handler(vmst, ir, pc)
		} else {
			// 如果未找到，则判定为非法指令
			rdid, rval, newPC, trap = handleIllegal(vmst, ir, pc)
		}

		// --- 3. 陷阱处理 (Trap Handling) ---
		if trap != 0 {
			if trap == TRAP_CODE_EXIT { // 特殊情况：通过 ecall 正常退出
				return trap
			}
			// 对于其他陷阱，保存当前的PC到mepc，
			// 将陷阱相关信息存入mcause和mtval，然后中断执行循环。
			vmst.Core.Mepc = pc
			vmst.Core.Mtval = ir // 对于指令相关的陷阱，通常将指令本身存入 mtval
			vmst.Core.Mcause = uint32(trap)
			break
		}

		// --- 4. 写回 (Write Back) ---
		// 如果指令需要写回结果到整数寄存器（rdid != 0），则执行写回操作。
		// x0 寄存器恒为0，不能被写入。
		if rdid != 0 {
			vmst.Core.Regs[rdid] = rval
		}

		// --- 5. 更新PC ---
		pc = newPC
	}

	// 将最终的PC值写回到核心状态中
	vmst.Core.PC = pc
	return trap
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
