package vm

// InstructionFunc 定义了所有指令处理函数的统一签名。
// 每个函数负责解码和执行一条指令。
//
// 参数:
//
//	vmst:  指向当前虚拟机状态的指针，包括寄存器和内存等核心组件。
//	ir:    从内存中获取的 32 位原始指令字。
//	pc:    当前指令的程序计数器（地址）。
//
// 返回值:
//
//	rdid:  目标寄存器的 ID（索引）。如果指令不写入通用寄存器，则为 0。
//	rval:  要写入目标寄存器的 32 位值。
//	newPC: 下一条指令的程序计数器。对于大多数指令，它是 pc + 4；对于跳转指令，它是目标地址。
//	trap:  如果在执行过程中发生异常（例如，非法指令、内存访问错误），则返回相应的陷阱代码；否则，返回 0。
type InstructionFunc func(vmst *VmState, ir uint32, pc uint32) (rdid uint32, rval uint32, newPC uint32, trap VmMcauseCode)

// OpiviFunc 是处理 OPIVI（向量-立即数）整数指令的函数类型。
type OpiviFunc func(vmst *VmState, vd, vs2, i, sew_bytes, imm, imm5 uint32)

// OpivxFunc 是处理 OPIVX（向量-标量）整数指令的函数类型。
type OpivxFunc func(vmst *VmState, vd, vs2, i, sew_bytes, op2 uint32)

// OpfvvFunc 定义了 OPFVV（向量-向量）浮点指令处理程序的签名。
type OpfvvFunc func(f1, f2 float32) float32

// OpfvfFunc 定义了 OPFVF（向量-标量）浮点指令处理程序的签名。
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
	// Instructions 是一个全局映射，将指令操作码映射到其对应的处理函数。
	// 这种设计允许 VM 的主执行循环根据操作码快速查找和调用正确的处理逻辑。
	Instructions = make(map[uint32]InstructionFunc)
)

// init 在包初始化期间自动调用。
// 其目的是填充 `instructions` 映射，为每个已实现的 RISC-V 操作码注册一个处理函数。
// 此处未注册的操作码将被视为非法指令。
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
	Instructions[OPCODE_FENCE] = handleMiscMem

	Instructions[OPCODE_LOAD_FP] = handleLoadFP   // F 扩展
	Instructions[OPCODE_STORE_FP] = handleStoreFP // F 扩展
	Instructions[OPCODE_OP_FP] = handleOpFP       // F 扩展
	Instructions[OPCODE_MADD] = handleFMA         // F 扩展 (FMA)
	Instructions[OPCODE_MSUB] = handleFMA         // F 扩展 (FMA)
	Instructions[OPCODE_NMSUB] = handleFMA        // F 扩展 (FMA)
	Instructions[OPCODE_NMADD] = handleFMA        // F 扩展 (FMA)
	Instructions[OPCODE_VECTOR] = handleVector    // V 扩展
	Instructions[OPCODE_AMO] = handleAMO          // A 扩展
}

// handleIllegal 是 `instructions` 映射中任何未注册操作码的回退处理程序。
// 它总是返回一个非法指令陷阱。
func handleIllegal(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, VmMcauseCode) {
	// 设置陷阱值（非法指令本身）
	if vmst != nil {
		vmst.Core.Mtval = ir
	}
	return 0, 0, pc, CAUSE_ILLEGAL_INSTRUCTION
}

// handleLUI 处理 LUI (加载高位立即数) 指令。
// LUI 指令将一个 20 位立即数加载到目标寄存器的高 20 位，并清除低 12 位。
// 格式: lui 目标寄存器, 立即数
func handleLUI(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, VmMcauseCode) {
	rdid := (ir >> 7) & 0x1f
	// U-Type 指令的立即数在 [31:12] 位
	rval := ir & 0xfffff000
	return rdid, rval, pc + 4, CAUSE_TRAP_CODE_OK
}

// handleAUIPC 处理 AUIPC (PC加高位立即数) 指令。
// AUIPC 指令将一个 20 位立即数（左移 12 位）加到当前 PC，并将结果存入目标寄存器。
// 它主要用于生成 PC 相关的地址。
// 格式: auipc 目标寄存器, 立即数
func handleAUIPC(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, VmMcauseCode) {
	rdid := (ir >> 7) & 0x1f
	imm := ir & 0xfffff000
	rval := pc + imm
	return rdid, rval, pc + 4, CAUSE_TRAP_CODE_OK
}

// handleJAL 处理 JAL (跳转并链接) 指令。
// JAL 指令将 pc+4 的值存入目标寄存器 rd，然后无条件跳转到 `pc + 20-bit signed offset`。
// 偏移量根据 J-Type 格式从指令字中重新组合。
// 格式: jal 目标寄存器, 偏移量
func handleJAL(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, VmMcauseCode) {
	rdid := (ir >> 7) & 0x1f
	rval := pc + 4

	var imm uint32
	imm |= ((ir >> 31) & 1) << 20
	imm |= ((ir >> 12) & 0xff) << 12
	imm |= ((ir >> 20) & 1) << 11
	imm |= ((ir >> 21) & 0x3ff) << 1

	if (imm & 0x100000) != 0 {
		imm |= 0xfff00000 // 符号扩展到 32 位 uint32
	}

	return rdid, rval, pc + imm, CAUSE_TRAP_CODE_OK
}

// handleJALR 处理 JALR (寄存器跳转并链接) 指令。
// JALR 指令将 pc+4 的值存入目标寄存器 rd，然后跳转到 `rs1 + 12-bit signed immediate` 的地址。
// 目标地址的最低有效位总是被清除。
// 格式: jalr 目标寄存器, 源寄存器1, 偏移量
func handleJALR(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, VmMcauseCode) {
	rs1id := (ir >> 15) & 0x1f
	rdid := (ir >> 7) & 0x1f
	imm := int32(ir&0xfff00000) >> 20 // 12位符号扩展
	rval := pc + 4
	newPC := (vmst.Core.Regs[rs1id] + uint32(imm)) & 0xfffffffe
	return rdid, rval, newPC, CAUSE_TRAP_CODE_OK
}

// handleBranch 处理所有条件分支指令 (BEQ, BNE, BLT, BGE, BLTU, BGEU)。
// 它比较寄存器 rs1 和 rs2 的值，如果条件满足，则跳转到 `pc + 12-bit signed offset`。
// 偏移量根据 B-Type 格式从指令字中重新组合。
// 格式: beq 源寄存器1, 源寄存器2, 偏移量
func handleBranch(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, VmMcauseCode) {
	rs1id := (ir >> 15) & 0x1f
	rs2id := (ir >> 20) & 0x1f
	funct3 := (ir >> 12) & 0x7

	rs1 := vmst.Core.Regs[rs1id]
	rs2 := vmst.Core.Regs[rs2id]
	var taken bool

	switch funct3 {
	case FUNCT3_BEQ:
		taken = (rs1 == rs2)
	case FUNCT3_BNE:
		taken = (rs1 != rs2)
	case FUNCT3_BLT:
		taken = (int32(rs1) < int32(rs2))
	case FUNCT3_BGE:
		taken = (int32(rs1) >= int32(rs2))
	case FUNCT3_BLTU:
		taken = (rs1 < rs2)
	case FUNCT3_BGEU:
		taken = (rs1 >= rs2)
	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}

	if taken {
		var imm uint32
		imm |= ((ir >> 31) & 1) << 12
		imm |= ((ir >> 7) & 1) << 11
		imm |= ((ir >> 25) & 0x3f) << 5
		imm |= ((ir >> 8) & 0xf) << 1
		if (imm & 0x1000) != 0 {
			imm |= 0xffffe000
		}
		return 0, 0, pc + imm, CAUSE_TRAP_CODE_OK
	}

	return 0, 0, pc + 4, CAUSE_TRAP_CODE_OK
}

// handleLoad 处理所有加载指令 (LB, LH, LW, LBU, LHU)。
// 它计算有效地址 `rs1 + offset`，从内存中读取数据，然后将其写入目标寄存器 rd。
// 它还执行内存边界检查和地址对齐检查。
// 格式: lw 目标寄存器, 偏移量(源寄存器1)
func handleLoad(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, VmMcauseCode) {
	rs1id := (ir >> 15) & 0x1f
	rdid := (ir >> 7) & 0x1f
	funct3 := (ir >> 12) & 0x7
	// 来自 I-Type 格式的 12 位立即数
	imm := int32(ir&0xfff00000) >> 20

	// 计算内存访问的绝对地址
	vaddr := vmst.Core.Regs[rs1id] + uint32(imm)
	paddr, trap := vmst.TranslateAddress(vaddr, VmMemAccessLoad)
	if trap != 0 {
		return 0, 0, 0, trap
	}

	var rval uint32
	var ok bool

	// 根据 funct3 执行具体的加载操作
	switch funct3 {
	case FUNCT3_LB: // LB (加载字节, 有符号)
		// 读取一个字节并进行符号扩展
		var b uint8
		b, ok = vmst.LoadUint8(paddr)
		if !ok {
			// 设置陷阱值（故障地址）
			switch vmst.Core.Privilege {
			case PRIV_MACHINE:
				vmst.Core.Mtval = vaddr
			case PRIV_SUPERVISOR:
				vmst.Core.Stval = vaddr
			default:
				// 用户模式，设置机器模式的值
				vmst.Core.Mtval = vaddr
			}
			return 0, 0, 0, CAUSE_LOAD_ACCESS_FAULT
		}
		rval = uint32(int8(b))
	case FUNCT3_LH: // LH (加载半字, 有符号)
		// 读取一个半字并进行符号扩展
		var b uint16
		b, ok = vmst.LoadUint16(paddr)
		if !ok {
			// 设置陷阱值（故障地址）
			switch vmst.Core.Privilege {
			case PRIV_MACHINE:
				vmst.Core.Mtval = vaddr
			case PRIV_SUPERVISOR:
				vmst.Core.Stval = vaddr
			default:
				// 用户模式，设置机器模式的值
				vmst.Core.Mtval = vaddr
			}
			return 0, 0, 0, CAUSE_LOAD_ACCESS_FAULT
		}
		rval = uint32(int16(b))
	case FUNCT3_LW: // LW (加载字)
		rval, ok = vmst.LoadUint32(paddr)
		if !ok {
			// 设置陷阱值（故障地址）
			switch vmst.Core.Privilege {
			case PRIV_MACHINE:
				vmst.Core.Mtval = vaddr
			case PRIV_SUPERVISOR:
				vmst.Core.Stval = vaddr
			default:
				// 用户模式，设置机器模式的值
				vmst.Core.Mtval = vaddr
			}
			return 0, 0, 0, CAUSE_LOAD_ACCESS_FAULT
		}
	case FUNCT3_LBU: // LBU (加载字节, 无符号)
		// 读取一个字节并进行零扩展
		var b uint8
		b, ok = vmst.LoadUint8(paddr)
		if !ok {
			// 设置陷阱值（故障地址）
			switch vmst.Core.Privilege {
			case PRIV_MACHINE:
				vmst.Core.Mtval = vaddr
			case PRIV_SUPERVISOR:
				vmst.Core.Stval = vaddr
			default:
				// 用户模式，设置机器模式的值
				vmst.Core.Mtval = vaddr
			}
			return 0, 0, 0, CAUSE_LOAD_ACCESS_FAULT
		}
		rval = uint32(b)
	case FUNCT3_LHU: // LHU (加载半字, 无符号)
		// 读取一个半字并进行零扩展
		var b uint16
		b, ok = vmst.LoadUint16(paddr)
		if !ok {
			// 设置陷阱值（故障地址）
			switch vmst.Core.Privilege {
			case PRIV_MACHINE:
				vmst.Core.Mtval = vaddr
			case PRIV_SUPERVISOR:
				vmst.Core.Stval = vaddr
			default:
				// 用户模式，设置机器模式的值
				vmst.Core.Mtval = vaddr
			}
			return 0, 0, 0, CAUSE_LOAD_ACCESS_FAULT
		}
		rval = uint32(b)
	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}
	return rdid, rval, pc + 4, CAUSE_TRAP_CODE_OK
}

// handleStore 处理所有存储指令 (SB, SH, SW)。
// 它计算有效地址 `rs1 + offset` 并将寄存器 rs2 的值写入内存。
// 它还执行内存边界检查和地址对齐检查。
// 格式: sw rs2, offset(rs1)
func handleStore(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, VmMcauseCode) {
	rs1id := (ir >> 15) & 0x1f
	rs2id := (ir >> 20) & 0x1f
	funct3 := (ir >> 12) & 0x7

	// 从 S-Type 格式中提取并重组 12 位有符号偏移量
	imm_11_5 := (ir >> 25) & 0x7f
	imm_4_0 := (ir >> 7) & 0x1f
	imm_unsigned := (imm_11_5 << 5) | imm_4_0
	imm := int32(imm_unsigned<<20) >> 20 // 符号扩展

	// 计算内存访问的绝对地址
	vaddr := vmst.Core.Regs[rs1id] + uint32(imm)
	paddr, trap := vmst.TranslateAddress(vaddr, VmMemAccessStore)
	if trap != 0 {
		return 0, 0, 0, trap
	}

	rs2 := vmst.Core.Regs[rs2id]
	var ok bool

	// 根据 funct3 执行具体的存储操作
	switch funct3 {
	case FUNCT3_SB: // SB (Store Byte)
		ok = vmst.PutUint8(paddr, byte(rs2))
		if !ok {
			// 设置陷阱值（故障地址）
			switch vmst.Core.Privilege {
			case PRIV_MACHINE:
				vmst.Core.Mtval = vaddr
			case PRIV_SUPERVISOR:
				vmst.Core.Stval = vaddr
			default:
				// 用户模式，设置机器模式的值
				vmst.Core.Mtval = vaddr
			}
			return 0, 0, 0, CAUSE_STORE_ACCESS_FAULT
		}
	case FUNCT3_SH: // SH (Store Half-word)
		ok = vmst.PutUint16(paddr, uint16(rs2))
		if !ok {
			// 设置陷阱值（故障地址）
			switch vmst.Core.Privilege {
			case PRIV_MACHINE:
				vmst.Core.Mtval = vaddr
			case PRIV_SUPERVISOR:
				vmst.Core.Stval = vaddr
			default:
				// 用户模式，设置机器模式的值
				vmst.Core.Mtval = vaddr
			}
			return 0, 0, 0, CAUSE_STORE_ACCESS_FAULT
		}
	case FUNCT3_SW: // SW (Store Word)
		ok = vmst.PutUint32(paddr, rs2)
		if !ok {
			// 设置陷阱值（故障地址）
			switch vmst.Core.Privilege {
			case PRIV_MACHINE:
				vmst.Core.Mtval = vaddr
			case PRIV_SUPERVISOR:
				vmst.Core.Stval = vaddr
			default:
				// 用户模式，设置机器模式的值
				vmst.Core.Mtval = vaddr
			}
			return 0, 0, 0, CAUSE_STORE_ACCESS_FAULT
		}
	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}
	// 存储指令不写入通用寄存器
	return 0, 0, pc + 4, CAUSE_TRAP_CODE_OK
}

// handleOpImm 处理所有立即数算术和逻辑指令 (ADDI, SLTI, XORI, SLLI 等)。
// 它对 rs1 和一个 12 位立即数进行操作，并将结果存入 rd。
// 格式: addi rd, rs1, immediate
func handleOpImm(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, VmMcauseCode) {
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
		if (ir & 0x40000000) != 0 { // SRAI (funct7 的一部分)
			rval = uint32(int32(rs1) >> shamt)
		} else { // SRLI
			rval = rs1 >> shamt
		}
	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}
	return rdid, rval, pc + 4, CAUSE_TRAP_CODE_OK
}

// handleOp 处理所有寄存器-寄存器算术和逻辑指令 (ADD, SUB, SLT, XOR, SLL 等)。
// 它对 rs1 和 rs2 寄存器进行操作，并将结果存入 rd。
// 格式: add rd, rs1, rs2
func handleOp(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, VmMcauseCode) {
	rs1id := (ir >> 15) & 0x1f
	rs2id := (ir >> 20) & 0x1f
	rdid := (ir >> 7) & 0x1f
	funct3 := (ir >> 12) & 0x7
	funct7 := (ir >> 25) & 0x7f

	rs1 := vmst.Core.Regs[rs1id]
	rs2 := vmst.Core.Regs[rs2id]
	var rval uint32

	if funct7 == FUNCT7_M { // M 扩展 (乘法/除法)
		switch funct3 {
		case FUNCT3_MUL: // MUL
			rval = rs1 * rs2
		case FUNCT3_MULH: // MULH (有符号)
			rval = uint32(int64(int32(rs1)) * int64(int32(rs2)) >> 32)
		case FUNCT3_MULHSU: // MULHSU (有符号 * 无符号)
			rval = uint32(int64(int32(rs1)) * int64(uint64(rs2)) >> 32)
		case FUNCT3_MULHU: // MULHU (无符号)
			rval = uint32(uint64(rs1) * uint64(rs2) >> 32)
		case FUNCT3_DIV: // DIV (有符号)
			if rs2 == 0 {
				rval = 0xffffffff // 除以零，返回 -1
			} else if int32(rs1) == -2147483648 && int32(rs2) == -1 {
				rval = rs1 // 溢出情况
			} else {
				rval = uint32(int32(rs1) / int32(rs2))
			}
		case FUNCT3_DIVU: // DIVU (无符号)
			if rs2 == 0 {
				rval = 0xffffffff // 除以零，返回 2^32-1
			} else {
				rval = rs1 / rs2
			}
		case FUNCT3_REM: // REM (有符号)
			if rs2 == 0 {
				rval = rs1
			} else if int32(rs1) == -2147483648 && int32(rs2) == -1 {
				rval = 0 // 溢出情况
			} else {
				rval = uint32(int32(rs1) % int32(rs2))
			}
		case FUNCT3_REMU: // REMU (无符号)
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
	return rdid, rval, pc + 4, CAUSE_TRAP_CODE_OK
}

// handleSystem 处理系统级指令，包括 ECALL、EBREAK 和 CSR（控制和状态寄存器）指令。
func handleSystem(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, VmMcauseCode) {
	rs1id := (ir >> 15) & 0x1f
	rdid := (ir >> 7) & 0x1f
	funct3 := (ir >> 12) & 0x7
	csr := (ir >> 20) & 0xfff

	switch funct3 {
	case FUNCT3_SYSTEM_ECALL_EBREAK:
		funct12 := (ir >> 20) & 0xfff
		switch funct12 {
		case 0: // ECALL
			// 根据当前特权级别确定异常代码
			switch vmst.Core.Privilege {
			case PRIV_USER:
				return 0, 0, 0, CAUSE_USER_ECALL
			case PRIV_SUPERVISOR:
				return 0, 0, 0, CAUSE_SUPERVISOR_ECALL
			default: // Machine
				return 0, 0, 0, CAUSE_MACHINE_ECALL
			}
		case 1: // EBREAK
			return 0, 0, 0, CAUSE_BREAKPOINT
		case FUNCT12_MRET:
			// 从 M-mode 陷阱返回
			// 1. 恢复特权级别
			prev_priv := (vmst.Core.Mstatus & MSTATUS_MPP) >> 11
			vmst.Core.Privilege = uint8(prev_priv)
			// 2. 恢复中断使能状态
			mpie := (vmst.Core.Mstatus >> 7) & 1
			vmst.Core.Mstatus = (vmst.Core.Mstatus & ^uint32(MSTATUS_MIE)) | (mpie << 3)
			// 3. 将 MSTATUS.MPP 设置为 U-mode (0)
			vmst.Core.Mstatus &= ^uint32(MSTATUS_MPP)
			// 4. 将 MSTATUS.MPIE 设置为 1
			vmst.Core.Mstatus |= (1 << 7)
			// 5. PC 跳转到 MEPC
			newPC := vmst.Core.Mepc
			return 0, 0, newPC, CAUSE_TRAP_CODE_OK
		case FUNCT12_SRET:
			// 从 S-mode 陷阱返回
			// 1. 恢复特权级别
			prev_priv := (vmst.Core.Sstatus & SSTATUS_SPP) >> 8
			vmst.Core.Privilege = uint8(prev_priv)
			// 2. 恢复中断使能状态
			spie := (vmst.Core.Sstatus >> 5) & 1
			vmst.Core.Sstatus = (vmst.Core.Sstatus & ^uint32(SSTATUS_SIE)) | (spie << 1)
			// 3. 将 SSTATUS.SPP 设置为 U-mode (0)
			vmst.Core.Sstatus &= ^uint32(SSTATUS_SPP)
			// 4. 将 SSTATUS.SPIE 设置为 1
			vmst.Core.Sstatus |= (1 << 5)
			// 5. PC 跳转到 SEPC
			newPC := vmst.Core.Sepc
			return 0, 0, newPC, CAUSE_TRAP_CODE_OK
		default:
			// 其他特权指令，如 WFI, SFENCE.VMA 等
			return 0, 0, pc + 4, CAUSE_TRAP_CODE_OK // 当前实现为 NOP
		}
	case FUNCT3_CSRRW, FUNCT3_CSRRS, FUNCT3_CSRRC, FUNCT3_CSRRWI, FUNCT3_CSRRSI, FUNCT3_CSRRCI:
		csr_val, ok := vmst.CsrRead(csr)
		if !ok {
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}

		rval := csr_val // CSR 指令总是先将旧值读入 rd
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
		}
		if !vmst.CsrWrite(csr, new_csr_val) {
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
		return rdid, rval, pc + 4, CAUSE_TRAP_CODE_OK
	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}
}

// handleMiscMem 处理杂项内存指令，包括 FENCE 和 FENCE.I。
// 这些指令用于在复杂的内存模型中强制执行内存访问的顺序。
func handleMiscMem(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, VmMcauseCode) {
	funct3 := (ir >> 12) & 0x7

	switch funct3 {
	case 0: // FENCE
		// --- FENCE 指令的完整作用 ---
		//
		// **目的**: FENCE 指令用于确保内存操作的顺序性，这在多核处理器系统和与 I/O 设备交互时至关重要。
		// 现代处理器为了优化性能，可能会对内存读写操作进行乱序执行（Out-of-Order Execution）。
		// 在单线程程序中，这种重排通常是不可见的，因为处理器会保证程序的最终结果与顺序执行一致。
		// 但在多线程或与外部设备交互时，一个核心的内存写入操作可能不会立即对其他核心或设备可见，
		// 从而导致数据不一致的问题。
		//
		// **工作原理**: FENCE 就像一个屏障。它强制要求在 FENCE 指令之前的所有内存操作
		// （由 pred 字段指定类型）必须在 FENCE 指令之后的所有内存操作（由 succ 字段指定类型）
		// 开始执行之前，对系统中的其他部分（其他核心、设备等）完全可见。
		//
		// **在当前模拟器中的实现**:
		// 这个模拟器是单核的，并且严格按程序顺序执行指令，没有乱序执行或多级缓存。
		// 任何内存写入都会立即反映在主内存中。因此，FENCE 指令所要保证的顺序性在这里是天然满足的。
		// 将其视为空操作（NOP）是正确且标准的简化实现。
		return 0, 0, pc + 4, CAUSE_TRAP_CODE_OK
	case 1: // FENCE.I
		// --- FENCE.I 指令的完整作用 ---
		//
		// **目的**: FENCE.I 用于同步数据内存的写入操作和指令缓存的获取操作。
		// 这在动态生成或修改代码的场景中非常关键，例如 JIT (Just-In-Time) 编译器或自我修改代码。
		//
		// **工作原理**: 当一个程序在内存中写入了新的指令后，需要确保这些新指令已经被写入主存，
		// 并且处理器的指令缓存（I-cache）中任何与该内存地址相关的旧指令都已失效。
		// FENCE.I 指令就是用来完成这个同步的。它会刷新指令缓存，强制处理器在执行后续指令时
		// 重新从主内存中获取，从而确保执行的是最新写入的代码。
		//
		// **在当前模拟器中的实现**:
		// 我们的模拟器没有实现分离的数据缓存和指令缓存。每次取指都是直接从 `vmst.Memory` 字节数组中读取。
		// 因此，不存在指令缓存与主存不一致的问题。任何对内存的写入都会立即对下一次取指可见。
		// 所以，FENCE.I 在此也可以安全地实现为空操作（NOP）。
		return 0, 0, pc + 4, CAUSE_TRAP_CODE_OK
	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}
}
