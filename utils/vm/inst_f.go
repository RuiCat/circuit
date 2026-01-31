package vm

import (
	"math"
)

// getFRegS 从一个64位浮点寄存器中提取一个 NaN-boxed 的 float32。
func getFRegS(vmst *VmState, frid uint32) float32 {
	// 单精度值存储在64位寄存器的低32位。
	return math.Float32frombits(uint32(vmst.Core.FRegs[frid]))
}

// setFRegS 将一个 float32 值通过 NaN-boxing 写入一个64位浮点寄存器。
func setFRegS(vmst *VmState, frid uint32, val float32) {
	// 根据RISC-V规范，将单精度值存储在64位寄存器中时，
	// 高32位应全为1。
	vmst.Core.FRegs[frid] = 0xffffffff00000000 | uint64(math.Float32bits(val))
}

// handleLoadFP 根据 funct3 字段分发浮点加载指令 (FLW, FLD)。
func handleLoadFP(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	funct3 := (ir >> 12) & 0x7

	switch funct3 {
	case FUNCT3_FLW: // FLW (Load Word for F-extension)
		return handleLoadFP_S(vmst, ir, pc)
	case FUNCT3_FLD: // FLD (Load Double for D-extension)
		// 调用 inst_d.go 中的处理器
		return handleLoadFP_D(vmst, ir, pc)
	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}
}

// handleLoadFP_S 处理 FLW (浮点加载单字) 指令。
func handleLoadFP_S(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	rs1id := (ir >> 15) & 0x1f
	rdid := (ir >> 7) & 0x1f
	imm := int32(ir&0xfff00000) >> 20
	addr := vmst.Core.Regs[rs1id] + uint32(imm)
	ofs_addr := addr - VmRamImageOffSet

	// 边界检查
	if addr < VmRamImageOffSet || ofs_addr+4 > vmst.VmMemorySize {
		vmst.Core.Mtval = addr
		return 0, 0, 0, CAUSE_LOAD_ACCESS_FAULT
	}
	// 对齐检查
	if ofs_addr&3 != 0 {
		vmst.Core.Mtval = addr
		return 0, 0, 0, CAUSE_LOAD_ADDRESS_MISALIGNED
	}
	// 从内存加载32位值
	setFRegS(vmst, rdid, math.Float32frombits(vmst.LoadUint32(ofs_addr)))
	// 浮点加载指令不写入通用寄存器，因此返回 rdid = 0。
	return 0, 0, pc + 4, 0
}

// handleStoreFP 根据 funct3 字段分发浮点存储指令 (FSW, FSD)。
func handleStoreFP(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	funct3 := (ir >> 12) & 0x7

	switch funct3 {
	case FUNCT3_FSW: // FSW (Store Word for F-extension)
		return handleStoreFP_S(vmst, ir, pc)
	case FUNCT3_FSD: // FSD (Store Double for D-extension)
		return handleStoreFP_D(vmst, ir, pc)
	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}
}

// handleStoreFP_S 处理 FSW (浮点存储单字) 指令。
func handleStoreFP_S(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	rs1id := (ir >> 15) & 0x1f
	rs2id := (ir >> 20) & 0x1f

	// S-Type 立即数重组
	imm_11_5 := (ir >> 25) & 0x7f
	imm_4_0 := (ir >> 7) & 0x1f
	imm_unsigned := (imm_11_5 << 5) | imm_4_0
	imm := int32(imm_unsigned<<20) >> 20

	addr := vmst.Core.Regs[rs1id] + uint32(imm)
	ofs_addr := addr - VmRamImageOffSet

	// 边界检查
	if addr < VmRamImageOffSet || ofs_addr+4 > vmst.VmMemorySize {
		vmst.Core.Mtval = addr
		return 0, 0, 0, CAUSE_STORE_ACCESS_FAULT
	}
	// 对齐检查
	if ofs_addr&3 != 0 {
		vmst.Core.Mtval = addr
		return 0, 0, 0, CAUSE_STORE_ADDRESS_MISALIGNED
	}

	vmst.PutUint32(ofs_addr, math.Float32bits(getFRegS(vmst, rs2id)))
	return 0, 0, pc + 4, 0
}

// handleOpFPHelper 是 handleOpFP 的辅助函数，处理 F/D 扩展的非转换、
// 非移动、非比较的指令。
func handleOpFPHelper(vmst *VmState, ir uint32, _ uint32, funct7 uint32,
	fs1, fs2 float32, fd1, fd2 float64) (uint64, bool) {
	var result_bits uint64
	var supported = true
	// 根据 funct7 执行 F/D 扩展的算术、求平方根、取最小/最大值和符号注入操作
	switch funct7 {
	// --- F/D 扩展：算术运算 ---
	case FUNCT7_FADD_S:
		result_f := fs1 + fs2
		result_bits = 0xffffffff00000000 | uint64(math.Float32bits(result_f))
	case FUNCT7_FADD_D:
		result_d := fd1 + fd2
		result_bits = math.Float64bits(result_d)
	case FUNCT7_FSUB_S:
		result_f := fs1 - fs2
		result_bits = 0xffffffff00000000 | uint64(math.Float32bits(result_f))
	case FUNCT7_FSUB_D:
		result_d := fd1 - fd2
		result_bits = math.Float64bits(result_d)
	case FUNCT7_FMUL_S:
		result_f := fs1 * fs2
		result_bits = 0xffffffff00000000 | uint64(math.Float32bits(result_f))
	case FUNCT7_FMUL_D:
		result_d := fd1 * fd2
		result_bits = math.Float64bits(result_d)
	case FUNCT7_FDIV_S:
		result_f := fs1 / fs2
		result_bits = 0xffffffff00000000 | uint64(math.Float32bits(result_f))
	case FUNCT7_FDIV_D:
		result_d := fd1 / fd2
		result_bits = math.Float64bits(result_d)
	case FUNCT7_FSQRT_S:
		rs2id := (ir >> 20) & 0x1f
		if rs2id != 0 {
			return 0, false
		}
		result_f := float32(math.Sqrt(float64(fs1)))
		result_bits = 0xffffffff00000000 | uint64(math.Float32bits(result_f))
	case FUNCT7_FSQRT_D:
		rs2id := (ir >> 20) & 0x1f
		if rs2id != 0 {
			return 0, false
		}
		result_d := math.Sqrt(fd1)
		result_bits = math.Float64bits(result_d)

	// --- F/D 扩展: 最小值/最大值 ---
	case FUNCT7_FMIN_MAX_S:
		var result_f float32
		funct3 := (ir >> 12) & 0x7
		switch funct3 {
		case FUNCT3_FMIN_S:
			if isNaN_S(fs1) && !isNaN_S(fs2) {
				result_f = fs2
			} else if !isNaN_S(fs1) && isNaN_S(fs2) {
				result_f = fs1
			} else if fs1 == fs2 && math.Signbit(float64(fs1)) { // -0.0 < +0.0
				result_f = fs1
			} else {
				result_f = float32(math.Min(float64(fs1), float64(fs2)))
			}
		case FUNCT3_FMAX_S:
			if isNaN_S(fs1) && !isNaN_S(fs2) {
				result_f = fs2
			} else if !isNaN_S(fs1) && isNaN_S(fs2) {
				result_f = fs1
			} else if fs1 == fs2 && !math.Signbit(float64(fs1)) { // +0.0 > -0.0
				result_f = fs1
			} else {
				result_f = float32(math.Max(float64(fs1), float64(fs2)))
			}
		default:
			return 0, false
		}
		result_bits = 0xffffffff00000000 | uint64(math.Float32bits(result_f))
	case FUNCT7_FMIN_MAX_D:
		var result_d float64
		funct3 := (ir >> 12) & 0x7
		switch funct3 {
		case FUNCT3_FMIN_D:
			if math.IsNaN(fd1) && !math.IsNaN(fd2) {
				result_d = fd2
			} else if !math.IsNaN(fd1) && math.IsNaN(fd2) {
				result_d = fd1
			} else if fd1 == fd2 && math.Signbit(fd1) {
				result_d = fd1
			} else {
				result_d = math.Min(fd1, fd2)
			}
		case FUNCT3_FMAX_D:
			if math.IsNaN(fd1) && !math.IsNaN(fd2) {
				result_d = fd2
			} else if !math.IsNaN(fd1) && math.IsNaN(fd2) {
				result_d = fd1
			} else if fd1 == fd2 && !math.Signbit(fd1) {
				result_d = fd1
			} else {
				result_d = math.Max(fd1, fd2)
			}
		default:
			return 0, false
		}
		result_bits = math.Float64bits(result_d)
	// --- F/D 扩展：符号注入 ---
	case FUNCT7_FSGNJ_S:
		fs1_bits := vmst.Core.FRegs[ir>>15&0x1f]
		fs2_bits := vmst.Core.FRegs[ir>>20&0x1f]
		sign2 := uint32(fs2_bits) & 0x80000000
		body1 := uint32(fs1_bits) & 0x7fffffff
		var res_u32 uint32
		funct3 := (ir >> 12) & 0x7
		switch funct3 {
		case FUNCT3_FSGNJ_S:
			res_u32 = body1 | sign2
		case FUNCT3_FSGNJN_S:
			res_u32 = body1 | (^sign2 & 0x80000000)
		case FUNCT3_FSGNJX_S:
			sign1 := uint32(fs1_bits) & 0x80000000
			res_u32 = body1 | (sign1 ^ sign2)
		default:
			return 0, false
		}
		result_bits = 0xffffffff00000000 | uint64(res_u32)
	case FUNCT7_FSGNJ_D:
		fs1_bits := vmst.Core.FRegs[ir>>15&0x1f]
		fs2_bits := vmst.Core.FRegs[ir>>20&0x1f]
		sign2 := fs2_bits & 0x8000000000000000
		body1 := fs1_bits & 0x7fffffffffffffff
		funct3 := (ir >> 12) & 0x7
		switch funct3 {
		case FUNCT3_FSGNJ_D:
			result_bits = body1 | sign2
		case FUNCT3_FSGNJN_D:
			result_bits = body1 | (^sign2 & 0x8000000000000000)
		case FUNCT3_FSGNJX_D:
			sign1 := fs1_bits & 0x8000000000000000
			result_bits = body1 | (sign1 ^ sign2)
		default:
			return 0, false
		}
	default:
		supported = false
	}
	return result_bits, supported
}

// handleOpFP 处理所有单精度和双精度浮点操作指令 (F/D 扩展)。
// 它解码 funct7, funct3 和 rs2 字段以确定具体的操作。
// 通过一个集中的 switch 语句处理所有情况。
func handleOpFP(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	rdid := (ir >> 7) & 0x1f
	rs1id := (ir >> 15) & 0x1f
	rs2id := (ir >> 20) & 0x1f
	funct3 := (ir >> 12) & 0x7
	funct7 := (ir >> 25) & 0x7f

	// 用于写入整数/浮点寄存器的变量
	var int_rdid, int_rval uint32
	var result_bits uint64

	// --- 提前提取所有可能的操作数 ---
	rs1_val := vmst.Core.Regs[rs1id] // 用于整数-浮点转换

	// --- 按需将位模式转换为浮点数 ---
	fs1 := getFRegS(vmst, rs1id)
	fs2 := getFRegS(vmst, rs2id)
	fd1 := getFRegD(vmst, rs1id)
	fd2 := getFRegD(vmst, rs2id)

	// 首先尝试处理 F/D 扩展的算术、求平方根、取最小/最大值和符号注入操作
	result_bits, ok := handleOpFPHelper(vmst, ir, pc, funct7, fs1, fs2, fd1, fd2)
	if ok {
		// 如果指令被 handleOpFPHelper 成功处理，则将结果写入浮点寄存器
		vmst.Core.FRegs[rdid] = result_bits
		return 0, 0, pc + 4, 0
	}

	switch funct7 {
	// --- F/D 扩展：转换操作 ---
	case FUNCT7_FCVT_W_S:
		var val int64
		switch rs2id {
		case FRS2_FCVT_W_S:
			val = int64(int32(fs1)) // 0
		case FRS2_FCVT_WU_S:
			val = int64(uint32(fs1)) // 1
		default:
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
		int_rdid, int_rval = rdid, uint32(val)
		return int_rdid, int_rval, pc + 4, 0
	case FUNCT7_FCVT_S_W:
		var result_f float32
		switch rs2id {
		case FRS2_FCVT_S_W:
			result_f = float32(int32(rs1_val)) // 0
		case FRS2_FCVT_S_WU:
			result_f = float32(rs1_val) // 1
		default:
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
		result_bits = 0xffffffff00000000 | uint64(math.Float32bits(result_f))
	case FUNCT7_FCVT_W_D:
		var val int64
		switch rs2id {
		case FRS2_FCVT_W_D:
			val = int64(int32(fd1)) // 0
		case FRS2_FCVT_WU_D:
			val = int64(uint32(fd1)) // 1
		default:
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
		int_rdid, int_rval = rdid, uint32(val)
		return int_rdid, int_rval, pc + 4, 0
	case FUNCT7_FCVT_D_W:
		var result_d float64
		switch rs2id {
		case FRS2_FCVT_D_W:
			result_d = float64(int32(rs1_val)) // 0
		case FRS2_FCVT_D_WU:
			result_d = float64(rs1_val) // 1
		default:
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
		result_bits = math.Float64bits(result_d)
	case FUNCT7_FCVT_S_D:
		if rs2id != FRS2_FCVT_S_D {
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
		result_f := float32(fd1)
		result_bits = 0xffffffff00000000 | uint64(math.Float32bits(result_f))
	case FUNCT7_FCVT_D_S:
		if rs2id != FRS2_FCVT_D_S {
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
		result_d := float64(fs1)
		result_bits = math.Float64bits(result_d)

	// --- F/D 扩展：比较运算 ---
	case FUNCT7_FEQ_FLT_FLE_S:
		var cmp_result bool
		switch funct3 {
		case FUNCT3_FEQ_S:
			cmp_result = (fs1 == fs2)
		case FUNCT3_FLT_S:
			cmp_result = (fs1 < fs2)
		case FUNCT3_FLE_S:
			cmp_result = (fs1 <= fs2)
		default:
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
		if cmp_result {
			int_rval = 1
		}
		int_rdid = rdid
		return int_rdid, int_rval, pc + 4, 0
	case FUNCT7_FEQ_FLT_FLE_D:
		var cmp_result bool
		switch funct3 {
		case FUNCT3_FEQ_D:
			cmp_result = (fd1 == fd2)
		case FUNCT3_FLT_D:
			cmp_result = (fd1 < fd2)
		case FUNCT3_FLE_D:
			cmp_result = (fd1 <= fd2)
		default:
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
		if cmp_result {
			int_rval = 1
		}
		int_rdid = rdid
		return int_rdid, int_rval, pc + 4, 0

	// --- F/D 扩展：移动和分类 ---
	case FUNCT7_FMV_X_W_FCLASS_S: // FMV.X.W 和 FCLASS.S
		switch funct3 {
		case FUNCT3_FMV_X_W: // FMV.X.W
			int_rdid, int_rval = rdid, uint32(vmst.Core.FRegs[rs1id])
			return int_rdid, int_rval, pc + 4, 0
		case FUNCT3_FCLASS_S: // FCLASS.S
			int_rdid, int_rval = rdid, classify_float32(fs1)
			return int_rdid, int_rval, pc + 4, 0
		default:
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
	case FUNCT7_FMV_W_X: // FMV.W.X
		result_bits = 0xffffffff00000000 | uint64(rs1_val)
	case FUNCT7_FMV_X_D: // FCLASS.D 和 FMV.X.D(RV64)
		switch funct3 {
		case FUNCT3_FCLASS_D: // FCLASS.D
			int_rdid, int_rval = rdid, classify_float64(fd1)
			return int_rdid, int_rval, pc + 4, 0
		case FUNCT3_FMV_X_D: // FMV.X.D (RV64D)
			// In RV32, this moves the lower 32 bits of the double-precision float
			int_rdid, int_rval = rdid, uint32(vmst.Core.FRegs[rs1id])
			return int_rdid, int_rval, pc + 4, 0
		default:
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
	case FUNCT7_FMV_D_X: // FMV.D.X (RV64D)
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION

	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}

	// 对于结果为浮点数的指令，写回浮点寄存器
	vmst.Core.FRegs[rdid] = result_bits
	return 0, 0, pc + 4, 0
}

func isNaN_S(f float32) bool {
	return f != f
}

func classify_float32(f float32) uint32 {
	bits := math.Float32bits(f)
	switch {
	case math.IsInf(float64(f), 1):
		return 1 << 7
	case math.IsInf(float64(f), -1):
		return 1 << 0
	case isNaN_S(f):
		if (bits>>22)&1 == 1 { // quiet NaN
			return 1 << 9
		}
		return 1 << 8 // signaling NaN
	case f == 0 && bits&0x80000000 != 0: // negative zero
		return 1 << 3
	case f == 0 && bits&0x80000000 == 0: // positive zero
		return 1 << 4
	case (bits & 0x7f800000) == 0: // subnormal
		if bits&0x80000000 != 0 {
			return 1 << 2 // negative subnormal
		}
		return 1 << 5 // positive subnormal
	default: // normal
		if bits&0x80000000 != 0 {
			return 1 << 1 // negative normal
		}
		return 1 << 6 // positive normal
	}
}

func classify_float64(f float64) uint32 {
	bits := math.Float64bits(f)
	switch {
	case math.IsInf(f, 1):
		return 1 << 7
	case math.IsInf(f, -1):
		return 1 << 0
	case math.IsNaN(f):
		if (bits>>51)&1 == 1 { // quiet NaN
			return 1 << 9
		}
		return 1 << 8 // signaling NaN
	case f == 0 && bits&0x8000000000000000 != 0: // negative zero
		return 1 << 3
	case f == 0 && bits&0x8000000000000000 == 0: // positive zero
		return 1 << 4
	case (bits & 0x7ff0000000000000) == 0: // subnormal
		if bits&0x8000000000000000 != 0 {
			return 1 << 2 // negative subnormal
		}
		return 1 << 5 // positive subnormal
	default: // normal
		if bits&0x8000000000000000 != 0 {
			return 1 << 1 // negative normal
		}
		return 1 << 6 // positive normal
	}
}

// handleFMA 处理所有浮点乘加（Fused Multiply-Add, FMA）指令。
// 这是一个 R4 类型的指令，使用三个源寄存器（rs1, rs2, rs3）。
// 格式: fmadd.s rd, rs1, rs2, rs3
func handleFMA(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	rdid := (ir >> 7) & 0x1f
	rs1id := (ir >> 15) & 0x1f
	rs2id := (ir >> 20) & 0x1f
	rs3id := (ir >> 27) & 0x1f // rs3 在 funct7 字段中编码
	opcode := ir & 0x7f
	fmt := (ir >> 25) & 0x3 // 00 for .S, 01 for .D

	var result_bits uint64

	switch fmt {
	case 0: // 单精度 (.S)
		fs1 := getFRegS(vmst, rs1id)
		fs2 := getFRegS(vmst, rs2id)
		fs3 := getFRegS(vmst, rs3id)
		var result_f float32

		switch opcode {
		case OPCODE_MADD:
			result_f = (fs1 * fs2) + fs3
		case OPCODE_MSUB:
			result_f = (fs1 * fs2) - fs3
		case OPCODE_NMSUB:
			result_f = -(fs1 * fs2) + fs3
		case OPCODE_NMADD:
			result_f = -(fs1 * fs2) - fs3
		default:
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
		result_bits = 0xffffffff00000000 | uint64(math.Float32bits(result_f))

	case 1: // 双精度 (.D)
		fd1 := getFRegD(vmst, rs1id)
		fd2 := getFRegD(vmst, rs2id)
		fd3 := getFRegD(vmst, rs3id)
		var result_d float64

		switch opcode {
		case OPCODE_MADD:
			result_d = (fd1 * fd2) + fd3
		case OPCODE_MSUB:
			result_d = (fd1 * fd2) - fd3
		case OPCODE_NMSUB:
			result_d = -(fd1 * fd2) + fd3
		case OPCODE_NMADD:
			result_d = -(fd1 * fd2) - fd3
		default:
			return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
		}
		result_bits = math.Float64bits(result_d)
	default:
		return 0, 0, 0, CAUSE_ILLEGAL_INSTRUCTION
	}

	vmst.Core.FRegs[rdid] = result_bits
	return 0, 0, pc + 4, 0
}
