package vm

// handleVector 是所有RISC-V向量（V扩展）指令的顶层分发函数。
// 它通过解码指令的 `funct3` 字段来确定具体的向量操作类型，
// 例如是向量-向量（OPIVV）、向量-立即数（OPIVI）、向量-标量（OPIVX），
// 还是向量配置指令（vsetvl/vsetvli），然后调用相应的子处理器。
func handleVector(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	funct3 := (ir >> 12) & 0x7
	var trap int32

	// 将当前指令存储在VM状态中。这是一种简化实现的方式，允许子处理器（如此处的移位操作）
	// 在需要时能访问到完整的指令编码，而无需在函数调用栈中层层传递。
	vmst.lastIR = ir

	switch funct3 {
	case FUNCT3_OPIVV: // 向量-向量整数运算
		trap = vmst.handleVFPOPIVV(ir) // 注意：当前实现中，整数和浮点共用一个OPIVV处理器入口，内部再区分
	case 0b001: // 向量-向量浮点运算 (OPFVV)
		trap = vmst.handleVFPOPIVV(ir)
	case FUNCT3_OPIVI: // 向量-立即数整数运算
		trap = vmst.handleOPIVI(ir)
	case FUNCT3_OPIVX: // 向量-标量整数运算
		trap = vmst.handleOPIVX(ir)
	case 0b101: // 向量-标量浮点运算 (OPFVF)
		trap = vmst.handleVFPOPIVF(ir)
	case FUNCT3_OP_V: // 向量配置指令 vsetvl/vsetvli
		trap = vmst.handleVSETVL(ir)
	default:
		// 非法的funct3值
		trap = CAUSE_ILLEGAL_INSTRUCTION
	}

	// 向量指令的结果通常写入向量寄存器，而不是通用寄存器，因此 rdid 和 rval 通常为0。
	// 如果发生陷阱，则返回陷阱原因；否则，PC前进4字节。
	if trap != 0 {
		return 0, 0, 0, trap
	}
	return 0, 0, pc + 4, 0
}

// handleVSETVL 实现了 `vsetvl` 和 `vsetvli` 指令，这是向量扩展的核心配置机制。
// 这条指令根据两个输入——期望的向量长度（avl, 来自rs1或zimm）和类型配置（vtypei, 来自rs2或imm）——
// 来设置两个关键的CSR：`vl`（实际向量长度）和 `vtype`（向量类型）。
func (vmst *VmState) handleVSETVL(ir uint32) int32 {
	// --- 解码指令字段 ---
	rdid := (ir >> 7) & 0x1f
	rs1id := (ir >> 15) & 0x1f
	imm12 := ir >> 20

	// vsetvli 指令使用 imm12 的一部分作为 vtypei，另一部分作为 avl。
	// vsetvl  指令使用 rs2 作为 vtypei，rs1 作为 avl。
	// 此处实现的是 vsetvli 的简化版本，同时从指令中提取 vtypei。
	if (imm12 >> 11) != 0 { // 检查 vsetvli 的编码是否合法
		return CAUSE_ILLEGAL_INSTRUCTION
	}
	vtypei := imm12 & 0x7ff // 从指令中提取 vtype 设置

	// --- 解析 vtypei ---
	vsew := (vtypei >> 2) & 0x7          // Standard Element Width (SEW)
	vlmul_encoded := vtypei & 0x3        // Vector Length Multiplier (LMUL)
	vill_bit_from_instr := (vtypei >> 7) // 非法位

	// --- 验证 SEW 和 LMUL 的合法性 ---
	const VLEN_BITS = 128 // 本实现的向量寄存器物理长度为128位
	sew_bits := uint32(8 << vsew)
	sew_is_valid := sew_bits == 8 || sew_bits == 16 || sew_bits == 32

	var lmul float32
	lmul_is_valid := true
	switch vlmul_encoded {
	case VLMUL_1:
		lmul = 1
	case VLMUL_2:
		lmul = 2
	case VLMUL_4:
		lmul = 4
	case VLMUL_8:
		lmul = 8
	default:
		lmul_is_valid = false
	}

	if vill_bit_from_instr != 0 || !sew_is_valid || !lmul_is_valid || lmul > 8 {
		vmst.Core.Vtype = 1 << 31
		vmst.Core.Vl = 0
		if rdid != 0 {
			vmst.Core.Regs[rdid] = 0
		}
		vmst.Core.Vstart = 0
		return 0
	}

	vmst.Core.Vtype = vtypei
	vmst.Core.Vstart = 0
	vlmax := uint32(float32(VLEN_BITS/sew_bits) * lmul)

	var avl uint32
	if rs1id == 0 {
		avl = (ir >> 15) & 0x1f
	} else {
		avl = vmst.Core.Regs[rs1id]
	}

	var new_vl uint32
	if rs1id == 0 && rdid == 0 {
		new_vl = vlmax
	} else {
		new_vl = avl
		if new_vl > vlmax {
			new_vl = vlmax
		}
	}
	vmst.Core.Vl = new_vl

	if rdid != 0 {
		vmst.Core.Regs[rdid] = new_vl
	}

	return 0
}
