package vm

// handleAMO 处理所有原子内存操作指令 (A 扩展)。
// 这些指令以原子方式读取、修改并写回内存位置。
// 格式: amoadd.w rd, rs2, (rs1)
func handleAMO(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	// --- 解码指令字段 ---
	rs1id := (ir >> 15) & 0x1f
	rs2id := (ir >> 20) & 0x1f
	rdid := (ir >> 7) & 0x1f
	funct3 := (ir >> 12) & 0x7
	// funct5 位于 funct7 的高5位
	funct5 := (ir >> 27) & 0x1f

	// --- 验证指令格式 ---
	// A 扩展只定义了 funct3 为 2 (width=32bit) 的情况
	if funct3 != FUNCT3_AMO_W {
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}

	// --- 内存地址计算与检查 ---
	addr := vmst.Core.Regs[rs1id]
	// 地址必须是4字节对齐的
	if addr&3 != 0 {
		vmst.Core.Mtval = addr
		return 0, 0, 0, CAUSE_STORE_ADDRESS_MISALIGNED
	}

	ofs_addr := addr - VmRamImageOffSet
	// 边界检查
	if addr < VmRamImageOffSet || ofs_addr+4 > vmst.VmMemorySize {
		vmst.Core.Mtval = addr
		return 0, 0, 0, CAUSE_STORE_ACCESS_FAULT
	}

	// 任何原子操作都会使 Load Reservation 失效
	// LR 和 SC 有特殊处理
	is_lr_sc := funct5 == FUNCT5_LR || funct5 == FUNCT5_SC
	if !is_lr_sc {
		vmst.Core.LoadReservation = 0
	}

	// --- 根据 funct5 执行具体操作 ---
	switch funct5 {
	case FUNCT5_LR:
		// LR.W (Load-Reserved Word)
		// 从内存加载值，设置保留地址，并将值写入 rd
		rval := vmst.Memory.LoadUint32(ofs_addr)
		vmst.Core.LoadReservation = addr
		return rdid, rval, pc + 4, 0

	case FUNCT5_SC:
		// SC.W (Store-Conditional Word)
		// 检查地址是否与保留地址匹配
		if addr == vmst.Core.LoadReservation {
			// 成功：将 rs2 的值写入内存，rd 置为0
			val_to_store := vmst.Core.Regs[rs2id]
			vmst.Memory.PutUint32(ofs_addr, val_to_store)
			vmst.Core.LoadReservation = 0 // 清除保留
			return rdid, 0, pc + 4, 0
		} else {
			// 失败：不写入内存，rd 置为1
			return rdid, 1, pc + 4, 0
		}

	default:
		// --- 其他原子操作 (Read-Modify-Write) ---
		// 1. 读取原始值
		original_val := vmst.Memory.LoadUint32(ofs_addr)
		rs2_val := vmst.Core.Regs[rs2id]
		var result uint32

		// 2. 根据 funct5 执行计算
		switch funct5 {
		case FUNCT5_AMOSWAP: // AMOSWAP.W
			result = rs2_val
		case FUNCT5_AMOADD: // AMOADD.W
			result = original_val + rs2_val
		case FUNCT5_AMOXOR: // AMOXOR.W
			result = original_val ^ rs2_val
		case FUNCT5_AMOAND: // AMOAND.W
			result = original_val & rs2_val
		case FUNCT5_AMOOR: // AMOOR.W
			result = original_val | rs2_val
		case FUNCT5_AMOMIN: // AMOMIN.W (signed)
			if int32(original_val) < int32(rs2_val) {
				result = original_val
			} else {
				result = rs2_val
			}
		case FUNCT5_AMOMAX: // AMOMAX.W (signed)
			if int32(original_val) > int32(rs2_val) {
				result = original_val
			} else {
				result = rs2_val
			}
		case FUNCT5_AMOMINU: // AMOMINU.W (unsigned)
			if original_val < rs2_val {
				result = original_val
			} else {
				result = rs2_val
			}
		case FUNCT5_AMOMAXU: // AMOMAXU.W (unsigned)
			if original_val > rs2_val {
				result = original_val
			} else {
				result = rs2_val
			}
		default:
			// 未知的 funct5
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}

		// 3. 将计算结果写回内存
		vmst.Memory.PutUint32(ofs_addr, result)

		// 4. 将原始值写入目标寄存器 rd
		return rdid, original_val, pc + 4, 0
	}
}
