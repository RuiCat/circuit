package vm

// CRegs 映射压缩寄存器编号 (0-7) 到标准寄存器索引 (x8-x15)
var CRegs = [8]uint32{8, 9, 10, 11, 12, 13, 14, 15}

// signExtend 对 n 位有符号数进行 32 位符号位扩展
func signExtend(val uint32, bits uint) uint32 {
	shift := 32 - bits
	return uint32(int32(val<<shift) >> shift)
}

// handleCompressed 是 16 位指令的处理入口
func handleCompressed(vmst *VmState, ir16 uint16, pc uint32) (rdid, rval, newPC uint32, trap int32) {
	// RISC-V 压缩指令通过低 2 位决定象限 (Quadrant)
	quadrant := ir16 & 0x3
	switch quadrant {
	case OPCODE_C0:
		return handleC0(vmst, ir16, pc)
	case OPCODE_C1:
		return handleC1(vmst, ir16, pc)
	case OPCODE_C2:
		return handleC2(vmst, ir16, pc)
	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}
}

// handleC0 处理象限 0：主要是基于基址寄存器的加载/存储指令
func handleC0(vmst *VmState, ir uint16, pc uint32) (uint32, uint32, uint32, int32) {
	funct3 := (ir >> 13) & 0x7
	rs1_p := (ir >> 7) & 0x7 // rd' / rs1'
	rd_p := (ir >> 2) & 0x7  // rd' / rs2'

	switch funct3 {
	case FUNCT3_C_ADDI4SPN: // C.ADDI4SPN: 立即数加到栈指针，结果存于 rd'
		// imm[9:2] 布局: 12:11=5:4, 10:7=9:6, 6=2, 5=3
		imm := (uint32(ir>>7) & 0x30) | (uint32(ir>>1) & 0x3C0) | ((uint32(ir) >> 2) & 8) | ((uint32(ir) >> 4) & 4)
		if imm == 0 {
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
		return CRegs[rd_p], vmst.Core.Regs[2] + imm, pc + 2, 0

	case FUNCT3_C_LW: // C.LW: 加载字
		// imm[6:2] 布局: 5=6, 10:12=5:3, 6=2
		offset := (uint32(ir>>7) & 0x38) | (uint32(ir>>4) & 0x4) | (uint32(ir<<1) & 0x40)
		addr := vmst.Core.Regs[CRegs[rs1_p]] + offset
		return performLoad(vmst, addr, CRegs[rd_p], pc)

	case FUNCT3_C_SW: // C.SW: 存储字
		offset := (uint32(ir>>7) & 0x38) | (uint32(ir>>4) & 0x4) | (uint32(ir<<1) & 0x40)
		addr := vmst.Core.Regs[CRegs[rs1_p]] + offset
		val := vmst.Core.Regs[CRegs[rd_p]]
		return performStore(vmst, addr, val, pc)

	case FUNCT3_C_FLD, FUNCT3_C_FSD: // C.FLD, C.FSD (RV64/128 专有，RV32 非法)
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION

	case FUNCT3_C_FLW, FUNCT3_C_FSW: // C.FLW, C.FSW (RV32 浮点暂占位实现)
		return 0, 0, pc + 2, 0

	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}
}

// handleC1 处理象限 1：算术、跳转和立即数指令
func handleC1(vmst *VmState, ir uint16, pc uint32) (uint32, uint32, uint32, int32) {
	funct3 := (ir >> 13) & 0x7
	rd_rs1 := (ir >> 7) & 0x1f
	imm6 := (uint32(ir>>12)&0x1)<<5 | uint32(ir>>2)&0x1f

	switch funct3 {
	case FUNCT3_C_NOP_ADDI: // C.ADDI / C.NOP
		if rd_rs1 == 0 {
			return 0, 0, pc + 2, 0 // C.NOP
		}
		imm := signExtend(imm6, 6)
		return uint32(rd_rs1), vmst.Core.Regs[rd_rs1] + imm, pc + 2, 0

	case FUNCT3_C_JAL: // C.JAL (RV32 专用)
		offset := decodeCJImm(ir)
		return 1, pc + 2, pc + offset, 0 // 返回地址存入 x1 (ra)

	case FUNCT3_C_LI: // C.LI (Load Immediate)
		if rd_rs1 == 0 {
			return 0, 0, pc + 2, 0 // HINT
		}
		return uint32(rd_rs1), signExtend(imm6, 6), pc + 2, 0

	case FUNCT3_C_LUI_ADDI16SP: // C.LUI / C.ADDI16SP
		if rd_rs1 == 2 { // C.ADDI16SP
			// 修正后的提取逻辑：
			imm := ((uint32(ir)>>12)&1)<<9 | // bit 9 (sign)
				((uint32(ir)>>6)&1)<<4 | // bit 4
				((uint32(ir)>>5)&1)<<6 | // bit 6
				((uint32(ir)>>3)&3)<<7 | // bits 8:7 (注意是 >> 3)
				((uint32(ir)>>2)&1)<<5 // bit 5

			imm_s := signExtend(imm, 10)
			// 注意：C.ADDI16SP 的立即数是 16 字节对齐的，所以要乘以 16
			// 但如果你的 signExtend 只是处理了位，这里需要手动补上最后的 0
			final_imm := int32(imm_s)
			if final_imm == 0 {
				return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
			}
			return 2, uint32(int32(vmst.Core.Regs[2]) + final_imm), pc + 2, 0
		} else { // C.LUI
			if rd_rs1 == 0 {
				return 0, 0, pc + 2, 0 // HINT
			}
			imm := signExtend(imm6, 6) << 12
			if imm == 0 {
				return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
			}
			return uint32(rd_rs1), imm, pc + 2, 0
		}

	case FUNCT3_C_MISC_ALU: // 杂项算术指令 (SRLI, SRAI, ANDI, 以及 R-type 压缩)
		subop := (ir >> 10) & 0x3
		rd_p := CRegs[(ir>>7)&0x7]
		switch subop {
		case FUNCT2_C_SRLI, FUNCT2_C_SRAI, FUNCT2_C_ANDI: // SRLI, SRAI, ANDI
			shamt := imm6
			if subop != FUNCT2_C_ANDI && shamt >= 32 {
				return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
			}
			var res uint32
			switch subop {
			case FUNCT2_C_SRLI:
				res = vmst.Core.Regs[rd_p] >> shamt // SRLI
			case FUNCT2_C_SRAI:
				res = uint32(int32(vmst.Core.Regs[rd_p]) >> shamt) // SRAI
			case FUNCT2_C_ANDI:
				res = vmst.Core.Regs[rd_p] & signExtend(imm6, 6) // ANDI
			}
			return rd_p, res, pc + 2, 0
		case FUNCT2_C_REG_ALU: // C.SUB, C.XOR, C.OR, C.AND
			rs2_p := CRegs[(ir>>2)&0x7]
			funct2 := (ir >> 5) & 0x3
			is_sub := (ir >> 12) & 0x1
			v1, v2 := vmst.Core.Regs[rd_p], vmst.Core.Regs[rs2_p]
			var res uint32
			if is_sub == 0 {
				switch funct2 {
				case 0:
					res = v1 - v2 // SUB
				case 1:
					res = v1 ^ v2 // XOR
				case 2:
					res = v1 | v2 // OR
				case 3:
					res = v1 & v2 // AND
				}
			} else {
				// 注意：在标准 C 扩展中，funct2 的 0-3 对应 SUB, XOR, OR, AND
				// 这里保留原始逻辑中的 switch 结构
				return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
			}
			return rd_p, res, pc + 2, 0
		}

	case FUNCT3_C_J: // C.J (无条件跳转)
		return 0, 0, pc + decodeCJImm(ir), 0

	case FUNCT3_C_BEQZ, FUNCT3_C_BNEZ: // C.BEQZ, C.BNEZ (条件分支)
		rs1_p := CRegs[(ir>>7)&0x7]
		offset := decodeCBImm(ir)
		cond := vmst.Core.Regs[rs1_p] == 0
		if funct3 == FUNCT3_C_BNEZ {
			cond = !cond // BNEZ
		}
		if cond {
			return 0, 0, pc + offset, 0
		}
		return 0, 0, pc + 2, 0
	}
	return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
}

// handleC2 处理象限 2：栈指针相关加载/存储、跳转和寄存器指令
func handleC2(vmst *VmState, ir uint16, pc uint32) (uint32, uint32, uint32, int32) {
	funct3 := (ir >> 13) & 0x7
	rd_rs1 := (uint32(ir) >> 7) & 0x1f
	rs2 := (uint32(ir) >> 2) & 0x1f

	switch funct3 {
	case FUNCT3_C_SLLI: // C.SLLI
		shamt := (uint32(ir>>12)&1)<<5 | rs2
		if rd_rs1 == 0 {
			return 0, 0, pc + 2, 0 // HINT
		}
		if shamt >= 32 {
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
		return rd_rs1, vmst.Core.Regs[rd_rs1] << shamt, pc + 2, 0

	case FUNCT3_C_LWSP: // C.LWSP (Load Word from Stack Pointer)
		rd := (uint32(ir) >> 7) & 0x1f
		if rd == 0 {
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
		// 立即数拼接:
		imm := ((uint32(ir)>>12)&1)<<5 |
			((uint32(ir)>>4)&7)<<2 |
			((uint32(ir)>>2)&3)<<6
		return performLoad(vmst, vmst.Core.Regs[2]+imm, rd, pc)

	case FUNCT3_C_JR_MV_ADD: // C.JR, C.MV, C.EBREAK, C.JALR, C.ADD
		bit12 := (ir >> 12) & 1
		if bit12 == 0 {
			if rs2 == 0 { // C.JR
				if rd_rs1 == 0 {
					return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
				}
				return 0, 0, vmst.Core.Regs[rd_rs1] & ^uint32(1), 0
			} else { // C.MV
				if rd_rs1 == 0 {
					return 0, 0, pc + 2, 0 // HINT
				}
				return rd_rs1, vmst.Core.Regs[rs2], pc + 2, 0
			}
		} else {
			if rd_rs1 == 0 && rs2 == 0 { // C.EBREAK
				return 0, 0, 0, CAUSE_BREAKPOINT
			} else if rs2 == 0 { // C.JALR
				if rd_rs1 == 0 {
					return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
				}
				target := vmst.Core.Regs[rd_rs1] & ^uint32(1)
				return 1, pc + 2, target, 0 // x1 = pc + 2
			} else { // C.ADD
				if rd_rs1 == 0 {
					return 0, 0, pc + 2, 0 // HINT
				}
				return rd_rs1, vmst.Core.Regs[rd_rs1] + vmst.Core.Regs[rs2], pc + 2, 0
			}
		}

	case FUNCT3_C_SWSP: // C.SWSP (Store Word to Stack Pointer)
		rs2 := (uint32(ir) >> 2) & 0x1f
		// imm[5:2] 在 inst[12:9], imm[7:6] 在 inst[8:7]
		imm := ((uint32(ir)>>9)&0xf)<<2 | // imm[5:2]
			((uint32(ir)>>7)&0x3)<<6 // imm[7:6]
		return performStore(vmst, vmst.Core.Regs[2]+imm, vmst.Core.Regs[rs2], pc)

	case FUNCT3_C_FLDSP, FUNCT3_C_FSDSP: // C.FLDSP, C.FSDSP (RV64 非法)
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	case FUNCT3_C_FLWSP, FUNCT3_C_FSWSP: // C.FLWSP, C.FSWSP (RV32 浮点占位)
		return 0, 0, pc + 2, 0

	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}
}

// --- 辅助解码函数 ---

// 解码 C.J 和 C.JAL 的立即数 (生成 imm[11:1]，bit 0 强制为 0)
func decodeCJImm(ir uint16) uint32 {
	// 指令位映射关系 (RISC-V Spec):
	// inst[12]   -> imm[11] (sign)
	// inst[11]   -> imm[4]
	// inst[10:9] -> imm[9:8]
	// inst[8]    -> imm[10]
	// inst[7]    -> imm[6]
	// inst[6]    -> imm[7]
	// inst[5:3]  -> imm[3:1]
	// inst[2]    -> imm[5]

	imm := ((uint32(ir)>>12)&1)<<11 | // bit 11
		((uint32(ir)>>11)&1)<<4 | // bit 4
		((uint32(ir)>>9)&3)<<8 | // bits 9:8
		((uint32(ir)>>8)&1)<<10 | // bit 10
		((uint32(ir)>>7)&1)<<6 | // bit 6
		((uint32(ir)>>6)&1)<<7 | // bit 7
		((uint32(ir)>>3)&7)<<1 | // bits 3:1
		((uint32(ir)>>2)&1)<<5 // bit 5

	// 此时 imm 的 bit 0 肯定是 0，因为它最小的左移是 << 1
	return signExtend(imm, 12)
}

// 解码 C.BEQZ 和 C.BNEZ 的立即数 (8-bit 有符号)
func decodeCBImm(ir uint16) uint32 {
	imm := ((ir>>12)&1)<<8 | // bit 8 (sign)
		((ir>>5)&3)<<6 | // bit 7:6
		((ir>>2)&1)<<5 | // bit 5
		((ir>>10)&3)<<3 | // bit 4:3
		((ir>>3)&3)<<1 // bit 2:1
	return signExtend(uint32(imm), 9)
}

// 抽象加载逻辑
func performLoad(vmst *VmState, addr uint32, rdid uint32, pc uint32) (uint32, uint32, uint32, int32) {
	paddr, trap := vmst.TranslateAddress(addr, VmMemAccessLoad)
	if trap != 0 {
		vmst.Core.Mtval = addr
		return 0, 0, 0, int32(trap)
	}
	if paddr&3 != 0 {
		vmst.Core.Mtval = addr
		return 0, 0, 0, CAUSE_LOAD_ADDRESS_MISALIGNED
	}
	val := vmst.LoadUint32(paddr)
	return rdid, val, pc + 2, 0
}

// 抽象存储逻辑
func performStore(vmst *VmState, addr uint32, val uint32, pc uint32) (uint32, uint32, uint32, int32) {
	paddr, trap := vmst.TranslateAddress(addr, VmMemAccessStore)
	if trap != 0 {
		vmst.Core.Mtval = addr
		return 0, 0, 0, int32(trap)
	}
	if paddr&3 != 0 {
		vmst.Core.Mtval = addr
		return 0, 0, 0, CAUSE_STORE_ADDRESS_MISALIGNED
	}
	vmst.PutUint32(paddr, val)
	return 0, 0, pc + 2, 0
}
