package vm

import (
	"encoding/binary"
)

// InstructionFunc 定义了所有指令处理函数的统一签名。
// 每个函数负责解码并执行一条指令。
//
// 参数:
//
//	vmst:  指向当前虚拟机状态的指针，包含了寄存器、内存等核心组件。
//	ir:    从内存中取出的32位原始指令字。
//	pc:    当前指令的程序计数器（地址）。
//
// 返回值:
//
//	rdid:  目标寄存器的ID（索引）。如果指令不写入通用寄存器，则为0。
//	rval:  要写入目标寄存器的32位值。
//	newPC: 下一条指令的程序计数器。对于大多数指令是 pc + 4，对于跳转指令则是目标地址。
//	trap:  如果执行过程中发生异常（例如非法指令、内存访问错误），则返回相应的陷阱代码；否则返回0。
type InstructionFunc func(vmst *VmState, ir uint32, pc uint32) (rdid uint32, rval uint32, newPC uint32, trap int32)

// OpiviFunc 是处理 OPIVI（向量-立即数）整数指令的函数类型。
type OpiviFunc func(vmst *VmState, vd, vs2, i, sew_bytes, imm, imm5 uint32)

// OpivxFunc 是处理 OPIVX（向量-标量）整数指令的函数类型。
type OpivxFunc func(vmst *VmState, vd, vs2, i, sew_bytes, op2 uint32)

// OpfvvFunc 定义了 OPFVV（向量-向量）浮点指令处理函数的签名。
type OpfvvFunc func(f1, f2 float32) float32

// OpfvfFunc 定义了 OPFVF（向量-标量）浮点指令处理函数的签名。
type OpfvfFunc func(f1, f2 float32) float32

var (
	// OpiviHandlers 将 funct6 操作码映射到其各自的 OPIVI 处理函数。
	OpiviHandlers = make(map[uint32]OpiviFunc)
	// OpivxHandlers 将 funct6 操作码映射到其各自的 OPIVX 处理函数。
	OpivxHandlers = make(map[uint32]OpivxFunc)
	// OpfvvHandlers 将 funct6 操作码映射到其各自的 OPFVV 处理函数。
	OpfvvHandlers = make(map[uint32]OpfvvFunc)
	// OpfvfHandlers 将 funct6 操作码映射到其各自的 OPFVF 处理函数。
	OpfvfHandlers = make(map[uint32]OpfvfFunc)
	// Instructions 是一个全局映射（map），将指令的操作码（opcode）映射到对应的处理函数。
	// 这种设计使得VM的主执行循环可以通过操作码快速查找并调用正确的处理逻辑。
	Instructions = make(map[uint32]InstructionFunc)
)

// init 函数在包初始化时被自动调用。
// 它的作用是填充 `instructions` 映射，为每个已实现的RISC-V操作码注册一个处理函数。
// 未在此处注册的操作码将被视作非法指令。
func init() {
	Instructions[OPCODE_LUI] = handleLUI
	Instructions[OPCODE_AUIPC] = handleAUIPC
	Instructions[OPCODE_JAL] = handleJAL
	Instructions[OPCODE_JALR] = handleJALR
	Instructions[OPCODE_BRANCH] = handleBranch
	Instructions[OPCODE_LOAD] = handleLoad
	Instructions[OPCODE_STORE] = handleStore
	Instructions[OPCODE_OP_IMM] = handleOpImm
	Instructions[OPCODE_OP] = handleOp
	Instructions[OPCODE_SYSTEM] = handleSystem
	Instructions[OPCODE_MISC_MEM] = handleMiscMem

	Instructions[OPCODE_LOAD_FP] = handleLoadFP   // F Extension
	Instructions[OPCODE_STORE_FP] = handleStoreFP // F Extension
	Instructions[OPCODE_OP_FP] = handleOpFP       // F Extension
	Instructions[OPCODE_MADD] = handleFMA         // F Extension (FMA)
	Instructions[OPCODE_MSUB] = handleFMA         // F Extension (FMA)
	Instructions[OPCODE_NMSUB] = handleFMA        // F Extension (FMA)
	Instructions[OPCODE_NMADD] = handleFMA        // F Extension (FMA)
	Instructions[OPCODE_VECTOR] = handleVector    // V Extension
	Instructions[OPCODE_AMO] = handleAMO          // A Extension
}

// handleIllegal 是一个“兜底”处理函数，用于处理任何未在 `instructions` 映射中注册的操作码。
// 它总是返回一个非法指令陷阱。
func handleIllegal(_ *VmState, _ uint32, pc uint32) (uint32, uint32, uint32, int32) {
	return 0, 0, pc, CAUSE_ILLEGAL_INSTRUCTION
}

// handleLUI 处理 LUI (Load Upper Immediate) 指令。
// LUI 指令将一个20位的立即数加载到目标寄存器的高20位，低12位清零。
// 格式: lui rd, immediate
func handleLUI(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	rdid := (ir >> 7) & 0x1f
	// U-Type 指令的立即数在 [31:12] 位
	rval := ir & 0xfffff000
	return rdid, rval, pc + 4, 0
}

// handleAUIPC 处理 AUIPC (Add Upper Immediate to PC) 指令。
// AUIPC 指令将一个20位立即数（左移12位）加到当前PC上，结果存入目标寄存器。
// 主要用于生成PC相关的地址。
// 格式: auipc rd, immediate
func handleAUIPC(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	rdid := (ir >> 7) & 0x1f
	imm := ir & 0xfffff000
	rval := pc + imm
	return rdid, rval, pc + 4, 0
}

// handleJAL 处理 JAL (Jump and Link) 指令。
// JAL 指令将 pc+4 的值存入目标寄存器rd，然后无条件跳转到 `pc + 20位有符号偏移`。
// 偏移量根据J-Type格式从指令字中重组。
// 格式: jal rd, offset
func handleJAL(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	rdid := (ir >> 7) & 0x1f
	rval := pc + 4

	// 从J-Type格式中提取和重组20位有符号偏移
	offset := (ir & 0x80000000) >> 11 // imm[20]
	offset |= (ir & 0x7fe00000) >> 20 // imm[10:1]
	offset |= (ir & 0x00100000) >> 9  // imm[11]
	offset |= (ir & 0x000ff000)       // imm[19:12]
	// 符号扩展
	if (offset & 0x00100000) != 0 {
		offset |= 0xfff00000
	}
	newPC := pc + offset
	return rdid, rval, newPC, 0
}

// handleJALR 处理 JALR (Jump and Link Register) 指令。
// JALR 指令将 pc+4 的值存入目标寄存器rd，然后跳转到 `rs1 + 12位有符号立即数` 的地址。
// 目标地址的最低位总是被清零。
// 格式: jalr rd, rs1, offset
func handleJALR(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	rs1id := (ir >> 15) & 0x1f
	rdid := (ir >> 7) & 0x1f
	// I-Type 格式的12位立即数
	imm := int32(ir&0xfff00000) >> 20
	rval := pc + 4
	// 计算跳转目标地址，并确保2字节对齐
	newPC := (vmst.Core.Regs[rs1id] + uint32(imm)) & 0xfffffffe
	return rdid, rval, newPC, 0
}

// handleBranch 处理所有条件分支指令 (BEQ, BNE, BLT, BGE, BLTU, BGEU)。
// 它比较rs1和rs2寄存器的值，如果条件满足，则跳转到 `pc + 12位有符号偏移`。
// 偏移量根据B-Type格式从指令字中重组。
// 格式: beq rs1, rs2, offset
func handleBranch(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	rs1id := (ir >> 15) & 0x1f
	rs2id := (ir >> 20) & 0x1f
	funct3 := (ir >> 12) & 0x7

	rs1 := vmst.Core.Regs[rs1id]
	rs2 := vmst.Core.Regs[rs2id]
	var taken bool

	// 根据 funct3 字段确定具体的分支条件
	switch funct3 {
	case FUNCT3_BEQ: // BEQ (Branch if Equal)
		taken = (rs1 == rs2)
	case FUNCT3_BNE: // BNE (Branch if Not Equal)
		taken = (rs1 != rs2)
	case FUNCT3_BLT: // BLT (Branch if Less Than, signed)
		taken = (int32(rs1) < int32(rs2))
	case FUNCT3_BGE: // BGE (Branch if Greater or Equal, signed)
		taken = (int32(rs1) >= int32(rs2))
	case FUNCT3_BLTU: // BLTU (Branch if Less Than, unsigned)
		taken = (rs1 < rs2)
	case FUNCT3_BGEU: // BGEU (Branch if Greater or Equal, unsigned)
		taken = (rs1 >= rs2)
	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}

	if taken {
		// 如果分支成功，计算跳转目标地址
		// 从B-Type格式中提取和重组12位有符号偏移
		offset := (ir & 0x80000000) >> 19 // imm[12]
		offset |= (ir & 0x7e000000) >> 20 // imm[10:5]
		offset |= (ir & 0x00000f80) >> 7  // imm[4:1]
		offset |= (ir & 0x00000080) << 4  // imm[11]
		// 符号扩展
		if (offset & 0x1000) != 0 {
			offset |= 0xffffe000
		}
		return 0, 0, pc + offset, 0
	}
	// 如果分支不成功，PC正常推进
	return 0, 0, pc + 4, 0
}

// handleLoad 处理所有加载指令 (LB, LH, LW, LBU, LHU)。
// 它计算有效地址 `rs1 + offset`，从内存中读取数据，然后写入目标寄存器rd。
// 它还执行内存边界检查和地址对齐检查。
// 格式: lw rd, offset(rs1)
func handleLoad(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	rs1id := (ir >> 15) & 0x1f
	rdid := (ir >> 7) & 0x1f
	funct3 := (ir >> 12) & 0x7
	// I-Type 格式的12位立即数
	imm := int32(ir&0xfff00000) >> 20

	// 计算内存访问的绝对地址
	addr := vmst.Core.Regs[rs1id] + uint32(imm)

	var rval uint32
	var access_size uint32

	// 根据 funct3 确定访问大小
	switch funct3 {
	case FUNCT3_LB, FUNCT3_LBU: // LB, LBU
		access_size = 1
	case FUNCT3_LH, FUNCT3_LHU: // LH, LHU
		access_size = 2
	case FUNCT3_LW: // LW
		access_size = 4
	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}

	// 将绝对地址转换为VM内存切片的偏移量
	ofs_addr := addr - VmRamImageOffSet
	// 边界检查：确保访问在有效的RAM范围内
	if addr < VmRamImageOffSet || ofs_addr+access_size > uint32(VmMemoRySize) {
		return 0, 0, 0, CAUSE_LOAD_ACCESS_FAULT
	}

	// 根据 funct3 执行具体的加载操作
	switch funct3 {
	case FUNCT3_LB: // LB (Load Byte, signed)
		// 读取字节并进行符号扩展
		rval = uint32(int8(vmst.Memory[ofs_addr]))
	case FUNCT3_LH: // LH (Load Half-word, signed)
		// 地址对齐检查
		if ofs_addr&1 != 0 {
			return 0, 0, 0, CAUSE_LOAD_ADDRESS_MISALIGNED
		}
		// 读取半字并进行符号扩展
		rval = uint32(int16(binary.LittleEndian.Uint16(vmst.Memory[ofs_addr:])))
	case FUNCT3_LW: // LW (Load Word)
		// 地址对齐检查
		if ofs_addr&3 != 0 {
			return 0, 0, 0, CAUSE_LOAD_ADDRESS_MISALIGNED
		}
		rval = binary.LittleEndian.Uint32(vmst.Memory[ofs_addr:])
	case FUNCT3_LBU: // LBU (Load Byte, unsigned)
		// 读取字节并进行零扩展
		rval = uint32(vmst.Memory[ofs_addr])
	case FUNCT3_LHU: // LHU (Load Half-word, unsigned)
		// 地址对齐检查
		if ofs_addr&1 != 0 {
			return 0, 0, 0, CAUSE_LOAD_ADDRESS_MISALIGNED
		}
		// 读取半字并进行零扩展
		rval = uint32(binary.LittleEndian.Uint16(vmst.Memory[ofs_addr:]))
	}
	return rdid, rval, pc + 4, 0
}

// handleStore 处理所有存储指令 (SB, SH, SW)。
// 它计算有效地址 `rs1 + offset`，并将rs2寄存器的值写入内存。
// 它还执行内存边界检查和地址对齐检查。
// 格式: sw rs2, offset(rs1)
func handleStore(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	rs1id := (ir >> 15) & 0x1f
	rs2id := (ir >> 20) & 0x1f
	funct3 := (ir >> 12) & 0x7

	// 从S-Type格式中提取和重组12位有符号偏移
	imm_11_5 := (ir >> 25) & 0x7f
	imm_4_0 := (ir >> 7) & 0x1f
	imm_unsigned := (imm_11_5 << 5) | imm_4_0
	imm := int32(imm_unsigned<<20) >> 20 // 符号扩展

	addr := vmst.Core.Regs[rs1id] + uint32(imm)
	rs2 := vmst.Core.Regs[rs2id]
	var access_size uint32

	// 根据 funct3 确定访问大小
	switch funct3 {
	case FUNCT3_SB: // SB
		access_size = 1
	case FUNCT3_SH: // SH
		access_size = 2
	case FUNCT3_SW: // SW
		access_size = 4
	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}

	// 边界检查
	ofs_addr := addr - VmRamImageOffSet
	if addr < VmRamImageOffSet || ofs_addr+access_size > uint32(VmMemoRySize) {
		return 0, 0, 0, CAUSE_STORE_ACCESS_FAULT
	}

	// 根据 funct3 执行具体的存储操作
	switch funct3 {
	case FUNCT3_SB: // SB (Store Byte)
		vmst.Memory[ofs_addr] = byte(rs2)
	case FUNCT3_SH: // SH (Store Half-word)
		if ofs_addr&1 != 0 {
			return 0, 0, 0, CAUSE_STORE_ADDRESS_MISALIGNED
		}
		binary.LittleEndian.PutUint16(vmst.Memory[ofs_addr:], uint16(rs2))
	case FUNCT3_SW: // SW (Store Word)
		if ofs_addr&3 != 0 {
			return 0, 0, 0, CAUSE_STORE_ADDRESS_MISALIGNED
		}
		binary.LittleEndian.PutUint32(vmst.Memory[ofs_addr:], rs2)
	}
	// 存储指令不写入通用寄存器
	return 0, 0, pc + 4, 0
}

// handleOpImm 处理所有立即数算术和逻辑指令 (ADDI, SLTI, XORI, SLLI 等)。
// 它对 rs1 和一个12位立即数进行操作，并将结果存入 rd。
// 格式: addi rd, rs1, immediate
func handleOpImm(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	rs1id := (ir >> 15) & 0x1f
	rdid := (ir >> 7) & 0x1f
	funct3 := (ir >> 12) & 0x7
	imm := int32(ir&0xfff00000) >> 20

	rs1 := vmst.Core.Regs[rs1id]
	var rval uint32

	switch funct3 {
	case FUNCT3_ADD_SUB: // ADDI
		rval = rs1 + uint32(imm)
	case FUNCT3_SLT: // SLTI
		if int32(rs1) < imm {
			rval = 1
		} else {
			rval = 0
		}
	case FUNCT3_SLTU: // SLTIU
		if rs1 < uint32(imm) {
			rval = 1
		} else {
			rval = 0
		}
	case FUNCT3_XOR: // XORI
		rval = rs1 ^ uint32(imm)
	case FUNCT3_OR: // ORI
		rval = rs1 | uint32(imm)
	case FUNCT3_AND: // ANDI
		rval = rs1 & uint32(imm)
	case FUNCT3_SLL: // SLLI
		shamt := (ir >> 20) & 0x1f
		rval = rs1 << shamt
	case FUNCT3_SRL_SRA: // SRLI / SRAI
		shamt := (ir >> 20) & 0x1f
		if (ir & 0x40000000) != 0 { // SRAI (funct7 a部分)
			rval = uint32(int32(rs1) >> shamt)
		} else { // SRLI
			rval = rs1 >> shamt
		}
	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}
	return rdid, rval, pc + 4, 0
}

// handleOp 处理所有寄存器-寄存器算术和逻辑指令 (ADD, SUB, SLT, XOR, SLL 等)。
// 它对 rs1 和 rs2 寄存器进行操作，并将结果存入 rd。
// 格式: add rd, rs1, rs2
func handleOp(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	rs1id := (ir >> 15) & 0x1f
	rs2id := (ir >> 20) & 0x1f
	rdid := (ir >> 7) & 0x1f
	funct3 := (ir >> 12) & 0x7
	funct7 := (ir >> 25) & 0x7f

	rs1 := vmst.Core.Regs[rs1id]
	rs2 := vmst.Core.Regs[rs2id]
	var rval uint32

	if funct7 == FUNCT7_M { // M 扩展 (乘除法)
		switch funct3 {
		case FUNCT3_MUL: // MUL
			rval = rs1 * rs2
		case FUNCT3_MULH: // MULH (signed)
			rval = uint32(int64(int32(rs1)) * int64(int32(rs2)) >> 32)
		case FUNCT3_MULHSU: // MULHSU (signed * unsigned)
			rval = uint32(int64(int32(rs1)) * int64(uint64(rs2)) >> 32)
		case FUNCT3_MULHU: // MULHU (unsigned)
			rval = uint32(uint64(rs1) * uint64(rs2) >> 32)
		case FUNCT3_DIV: // DIV (signed)
			if rs2 == 0 {
				rval = 0xffffffff // 除以零，返回-1
			} else if int32(rs1) == -2147483648 && int32(rs2) == -1 {
				rval = rs1 // 溢出情况
			} else {
				rval = uint32(int32(rs1) / int32(rs2))
			}
		case FUNCT3_DIVU: // DIVU (unsigned)
			if rs2 == 0 {
				rval = 0xffffffff // 除以零，返回 2^32-1
			} else {
				rval = rs1 / rs2
			}
		case FUNCT3_REM: // REM (signed)
			if rs2 == 0 {
				rval = rs1
			} else if int32(rs1) == -2147483648 && int32(rs2) == -1 {
				rval = 0 // 溢出情况
			} else {
				rval = uint32(int32(rs1) % int32(rs2))
			}
		case FUNCT3_REMU: // REMU (unsigned)
			if rs2 == 0 {
				rval = rs1
			} else {
				rval = rs1 % rs2
			}
		default:
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
	} else { // 基本整数指令
		switch funct3 {
		case FUNCT3_ADD_SUB: // ADD / SUB
			if funct7 == FUNCT7_SUB { // SUB
				rval = rs1 - rs2
			} else { // ADD
				rval = rs1 + rs2
			}
		case FUNCT3_SLL: // SLL
			rval = rs1 << (rs2 & 0x1f)
		case FUNCT3_SLT: // SLT
			if int32(rs1) < int32(rs2) {
				rval = 1
			}
		case FUNCT3_SLTU: // SLTU
			if rs1 < rs2 {
				rval = 1
			}
		case FUNCT3_XOR: // XOR
			rval = rs1 ^ rs2
		case FUNCT3_SRL_SRA: // SRL / SRA
			if funct7 == FUNCT7_SRA { // SRA
				rval = uint32(int32(rs1) >> (rs2 & 0x1f))
			} else { // SRL
				rval = rs1 >> (rs2 & 0x1f)
			}
		case FUNCT3_OR: // OR
			rval = rs1 | rs2
		case FUNCT3_AND: // AND
			rval = rs1 & rs2
		default:
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
	}
	return rdid, rval, pc + 4, 0
}

// handleSystem 处理系统级指令，包括 ECALL, EBREAK, 和 CSR (Control and Status Register) 指令。
func handleSystem(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	csr := (ir >> 20) & 0xfff
	// 特殊编码用于 ECALL 和 EBREAK
	if csr == 0 && ((ir>>7)&0x1f) == 0 && ((ir>>15)&0x1f) == 0 {
		if (ir >> 12) == 0 { // ECALL
			// a7 is x17, a0 is x10
			syscall_num := vmst.Core.Regs[17]
			switch syscall_num {
			case 93: // standard exit
				return 0, 0, pc, TRAP_CODE_EXIT
			case VmSysCallHalt:
				// The Run loop in vm.go handles this by setting VmStatusEnded
				return 0, 0, 0, CAUSE_USER_ECALL
			case VmSysCallYield:
				// For a simple VM, yield might not do anything special.
				// In a multitasking scenario, this would trigger a context switch.
				// Here, we can just treat it as a NOP and continue.
				return 0, 0, pc + 4, 0
			default:
				// Trigger a syscall event for the host to handle
				return 0, 0, 0, CAUSE_USER_ECALL
			}
		} else { // EBREAK
			return 0, 0, 0, CAUSE_BREAKPOINT
		}
	}

	rs1id := (ir >> 15) & 0x1f
	rdid := (ir >> 7) & 0x1f
	funct3 := (ir >> 12) & 0x7

	csr_val, ok := vmst.CsrRead(csr)
	if !ok {
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}

	rval := csr_val // CSR 指令总是先读取旧值到 rd
	var new_csr_val uint32

	switch funct3 {
	case FUNCT3_CSRRW: // CSRRW (Atomic Read/Write CSR)
		new_csr_val = vmst.Core.Regs[rs1id]
	case FUNCT3_CSRRS: // CSRRS (Atomic Read and Set Bits in CSR)
		new_csr_val = csr_val | vmst.Core.Regs[rs1id]
	case FUNCT3_CSRRC: // CSRRC (Atomic Read and Clear Bits in CSR)
		new_csr_val = csr_val &^ vmst.Core.Regs[rs1id]
	case FUNCT3_CSRRWI: // CSRRWI (立即数版本)
		zimm := rs1id
		new_csr_val = uint32(zimm)
	case FUNCT3_CSRRSI: // CSRRSI
		zimm := rs1id
		new_csr_val = csr_val | uint32(zimm)
	case FUNCT3_CSRRCI: // CSRRCI
		zimm := rs1id
		new_csr_val = csr_val &^ uint32(zimm)
	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}

	if !vmst.CsrWrite(csr, new_csr_val) {
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}

	return rdid, rval, pc + 4, 0
}

// handleMiscMem 处理杂项内存指令，当前主要是 FENCE。
// 在这个简单的模拟器中，FENCE 指令被当作一个空操作（NOP）。
func handleMiscMem(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	// FENCE is a NOP for this simulator.
	return 0, 0, pc + 4, 0
}
