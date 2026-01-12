package vm

import (
	"encoding/binary"
	"math"
)

// VmInaState 定义了 RISC-V 虚拟机核心的状态。
// 这个结构包含了整数、浮点和向量寄存器，
// 以及程序计数器 (PC) 和各种控制和状态寄存器 (CSR)。
type VmInaState struct {
	// Regs 是一个包含32个32位通用整数寄存器的数组。
	Regs [32]uint32
	// Fregs 是一个包含32个32位浮点寄存器的数组。
	Fregs [32]uint32
	// Vregs 是一个512字节的数组，用作向量寄存器文件。
	// 它被组织为32个128位（16字节）的向量寄存器。
	Vregs [512]byte

	// PC (程序计数器) 保存将要执行的下一条指令的地址。
	PC uint32
	// Mstatus 寄存器保存了处理器的当前状态。
	Mstatus uint32
	// Mscratch 是一个在机器模式陷阱处理程序中使用的临时寄存器。
	Mscratch uint32
	// Mtvec 保存了机器模式陷阱向量的基地址。
	Mtvec uint32
	// Mie 寄存器控制机器级中断的启用。
	Mie uint32
	// Mip 寄存器保存了挂起的机器级中断。
	Mip uint32
	// Mepc 是一个在发生异常时保存指令地址的寄存器。
	Mepc uint32
	// Mtval 寄存器保存了与陷阱相关的附加信息（例如，导致故障的地址）。
	Mtval uint32
	// Mcause 寄存器指示了发生陷阱的原因。
	Mcause uint32
	// Fcsr 是浮点控制和状态寄存器。
	Fcsr uint32

	// --- 向量扩展控制和状态寄存器 ---

	// Vstart 保存向量指令中要处理的起始元素索引。
	Vstart uint32
	// Vl (向量长度) 寄存器保存了当前向量操作中的元素数量。
	Vl uint32
	// Vtype (向量类型) 寄存器配置了向量元素的类型和分组。
	Vtype uint32

	// Extraflags 用于虚拟机特定的标志。
	Extraflags uint32
}

// VmImaStep 在虚拟机中执行指定数量的指令。
// 这是核心的指令执行循环，处理RV32IMAFV指令。
// 参数:
//
//	count: 要执行的最大指令数。
//
// 返回:
//
//	一个陷阱代码，如果发生异常则为非零值，否则为零。
func (vmst *VmState) VmImaStep(count int) int32 {
	var trap int32 = 0
	var rval uint32 = 0
	pc := vmst.Core.PC

	// 循环执行指定数量的指令
	for icount := 0; icount < count; icount++ {
		// --- 指令获取 ---
		// 检查PC是否对齐以及是否在内存边界内
		ofs_pc := pc - VmRamImageOffSet
		if ofs_pc >= VmMemoRySize {
			trap = CAUSE_INSTRUCTION_ACCESS_FAULT
			break
		}
		if ofs_pc&3 != 0 {
			trap = CAUSE_INSTRUCTION_ADDRESS_MISALIGNED
			break
		}

		// 从内存中获取指令
		ir := binary.LittleEndian.Uint32(vmst.Memory[ofs_pc:])
		// 默认情况下，目标寄存器索引和值
		rdid := (ir >> 7) & 0x1f
		rval = 0

		// --- 指令解码和执行 ---
		// 根据操作码（最低7位）进行切换
		switch ir & 0x7f {
		case OPCODE_LUI: // LUI (加载高位立即数)
			rval = ir & 0xfffff000
		case OPCODE_AUIPC: // AUIPC (将高位立即数加到 PC)
			rval = pc + (ir & 0xfffff000)
		case OPCODE_JAL: // JAL (跳转并链接)
			reladdy := ((ir & 0x80000000) >> 11) | ((ir & 0x7fe00000) >> 20) | ((ir & 0x00100000) >> 9) | (ir & 0x000ff000)
			if (reladdy & 0x00100000) != 0 {
				reladdy |= 0xffe00000 // 符号扩展
			}
			rval = pc + 4
			pc += reladdy - 4
		case OPCODE_JALR: // JALR (间接跳转并链接)
			imm := ir >> 20
			imm_se := uint32(int32(imm<<20) >> 20)
			rval = pc + 4
			pc = ((vmst.Core.Regs[(ir>>15)&0x1f] + imm_se) &^ 1) - 4
		case OPCODE_BRANCH: // 分支指令
			immm4 := ((ir & 0xf00) >> 7) | ((ir & 0x7e000000) >> 20) | ((ir & 0x80) << 4) | ((ir >> 31) << 12)
			if (immm4 & 0x1000) != 0 {
				immm4 |= 0xffffe000
			}
			rs1 := int32(vmst.Core.Regs[(ir>>15)&0x1f])
			rs2 := int32(vmst.Core.Regs[(ir>>20)&0x1f])
			immm4 = pc + immm4 - 4
			rdid = 0
			switch (ir >> 12) & 0x7 {
			case FUNCT3_BEQ:
				if rs1 == rs2 {
					pc = immm4
				}
			case FUNCT3_BNE:
				if rs1 != rs2 {
					pc = immm4
				}
			case FUNCT3_BLT:
				if rs1 < rs2 {
					pc = immm4
				}
			case FUNCT3_BGE:
				if rs1 >= rs2 {
					pc = immm4
				}
			case FUNCT3_BLTU:
				if uint32(rs1) < uint32(rs2) {
					pc = immm4
				}
			case FUNCT3_BGEU:
				if uint32(rs1) >= uint32(rs2) {
					pc = immm4
				}
			default:
				trap = CAUSE_ILLEGAL_INSTRUCTION
			}
		case OPCODE_LOAD_FP: // 浮点加载 (FLW)
			if (ir>>12)&FUNCT3_FLW == FUNCT3_FLW { // FLW, funct3=2
				rs1 := vmst.Core.Regs[(ir>>15)&0x1f]
				imm := ir >> 20
				imm_se := uint32(int32(imm<<20) >> 20)
				addr := rs1 + imm_se

				frd := (ir >> 7) & 0x1f
				rdid = 0 // 不是整数寄存器目标

				var loaded_val uint32
				if minirv32_mmio_range(addr) {
					loaded_val = vmst.ExtramLoad(addr, 2) // 访问类型 2 (字)
				} else {
					addr -= VmRamImageOffSet
					if addr >= VmMemoRySize-3 {
						trap = CAUSE_LOAD_ACCESS_FAULT
						rval = addr + VmRamImageOffSet
						break
					}
					loaded_val = binary.LittleEndian.Uint32(vmst.Memory[addr:])
				}
				vmst.Core.Fregs[frd] = loaded_val
			} else {
				trap = CAUSE_ILLEGAL_INSTRUCTION
			}
		case OPCODE_LOAD: // 加载
			rs1 := vmst.Core.Regs[(ir>>15)&0x1f]
			imm := ir >> 20
			imm_se := uint32(int32(imm<<20) >> 20)
			rsval := rs1 + imm_se

			if minirv32_mmio_range(rsval) {
				rval = vmst.ExtramLoad(rsval, (ir>>12)&0x7)
			} else {
				rsval -= VmRamImageOffSet
				if rsval >= VmMemoRySize-3 {
					trap = CAUSE_LOAD_ACCESS_FAULT
					rval = rsval + VmRamImageOffSet
				} else {
					switch (ir >> 12) & 0x7 {
					case FUNCT3_LB:
						rval = uint32(int8(vmst.Memory[rsval]))
					case FUNCT3_LH:
						rval = uint32(int16(binary.LittleEndian.Uint16(vmst.Memory[rsval:])))
					case FUNCT3_LW:
						rval = binary.LittleEndian.Uint32(vmst.Memory[rsval:])
					case FUNCT3_LBU:
						rval = uint32(vmst.Memory[rsval])
					case FUNCT3_LHU:
						rval = uint32(binary.LittleEndian.Uint16(vmst.Memory[rsval:]))
					default:
						trap = CAUSE_ILLEGAL_INSTRUCTION
					}
				}
			}
		case OPCODE_STORE_FP: // 浮点存储 (FSW)
			if (ir>>12)&FUNCT3_FSW == FUNCT3_FSW { // FSW, funct3=2
				rs1 := vmst.Core.Regs[(ir>>15)&0x1f]
				frs2 := (ir >> 20) & 0x1f

				imm_s := ((ir >> 25) << 5) | ((ir >> 7) & 0x1f)
				imm_se := uint32(int32(imm_s<<20) >> 20)
				addr := rs1 + imm_se

				rdid = 0

				val_to_store := vmst.Core.Fregs[frs2]

				if minirv32_mmio_range(addr) {
					vmst.extramStore(addr, val_to_store, 2) // 访问类型 2 (字)
				} else {
					addr -= VmRamImageOffSet
					if addr >= VmMemoRySize-3 {
						trap = CAUSE_STORE_AMO_ACCESS_FAULT
						rval = addr + VmRamImageOffSet
					} else {
						binary.LittleEndian.PutUint32(vmst.Memory[addr:], val_to_store)
					}
				}
			} else {
				trap = CAUSE_ILLEGAL_INSTRUCTION
			}
		case OPCODE_STORE: // 存储
			rs1 := vmst.Core.Regs[(ir>>15)&0x1f]
			rs2 := vmst.Core.Regs[(ir>>20)&0x1f]
			imm_s := ((ir >> 25) << 5) | ((ir >> 7) & 0x1f)
			imm_se := uint32(int32(imm_s<<20) >> 20)
			addy := rs1 + imm_se
			rdid = 0

			if minirv32_mmio_range(addy) {
				vmst.extramStore(addy, rs2, (ir>>12)&0x7)
			} else {
				addy -= VmRamImageOffSet
				if addy >= VmMemoRySize-3 {
					trap = CAUSE_STORE_AMO_ACCESS_FAULT
					rval = addy + VmRamImageOffSet
				} else {
					switch (ir >> 12) & 0x7 {
					case FUNCT3_SB:
						vmst.Memory[addy] = byte(rs2)
					case FUNCT3_SH:
						binary.LittleEndian.PutUint16(vmst.Memory[addy:], uint16(rs2))
					case FUNCT3_SW:
						binary.LittleEndian.PutUint32(vmst.Memory[addy:], rs2)
					default:
						trap = CAUSE_ILLEGAL_INSTRUCTION
					}
				}
			}
		case OPCODE_OP_IMM, OPCODE_OP: // 立即数操作和寄存器操作
			rs1 := vmst.Core.Regs[(ir>>15)&0x1f]
			is_reg := (ir & 0x20) != 0
			var rs2 uint32
			if is_reg {
				rs2 = vmst.Core.Regs[(ir>>20)&0x1f]
			} else {
				imm := ir >> 20
				rs2 = uint32(int32(imm<<20) >> 20) // 立即数符号扩展
			}

			if is_reg && (ir&0x02000000) != 0 { // RV32M (乘除法扩展)
				switch (ir >> 12) & 7 {
				case FUNCT3_MUL:
					rval = rs1 * rs2
				case FUNCT3_MULH:
					rval = uint32(int64(int32(rs1)) * int64(int32(rs2)) >> 32)
				case FUNCT3_MULHSU:
					rval = uint32(int64(int32(rs1)) * int64(int64(rs2)) >> 32)
				case FUNCT3_MULHU:
					rval = uint32(uint64(rs1) * uint64(rs2) >> 32)
				case FUNCT3_DIV:
					if rs2 == 0 {
						rval = 0xffffffff
					} else if int32(rs1) == math.MinInt32 && int32(rs2) == -1 {
						rval = rs1
					} else {
						rval = uint32(int32(rs1) / int32(rs2))
					}
				case FUNCT3_DIVU:
					if rs2 == 0 {
						rval = 0xffffffff
					} else {
						rval = rs1 / rs2
					}
				case FUNCT3_REM:
					if rs2 == 0 {
						rval = rs1
					} else if int32(rs1) == math.MinInt32 && int32(rs2) == -1 {
						rval = 0
					} else {
						rval = uint32(int32(rs1) % int32(rs2))
					}
				case FUNCT3_REMU:
					if rs2 == 0 {
						rval = rs1
					} else {
						rval = rs1 % rs2
					}
				}
			} else {
				switch (ir >> 12) & 7 {
				case FUNCT3_ADD_SUB:
					if is_reg && (ir&0x40000000) != 0 {
						rval = rs1 - rs2 // SUB
					} else {
						rval = rs1 + rs2 // ADD or ADDI
					}
				case FUNCT3_SLL:
					rval = rs1 << (rs2 & 0x1F)
				case FUNCT3_SLT:
					if int32(rs1) < int32(rs2) {
						rval = 1
					} else {
						rval = 0
					}
				case FUNCT3_SLTU:
					if rs1 < rs2 {
						rval = 1
					} else {
						rval = 0
					}
				case FUNCT3_XOR:
					rval = rs1 ^ rs2
				case FUNCT3_SRL_SRA:
					if (ir & 0x40000000) != 0 {
						rval = uint32(int32(rs1) >> (rs2 & 0x1F)) // SRA
					} else {
						rval = rs1 >> (rs2 & 0x1F) // SRL
					}
				case FUNCT3_OR:
					rval = rs1 | rs2
				case FUNCT3_AND:
					rval = rs1 & rs2
				}
			}
		case OPCODE_OP_FP: // 单精度浮点操作
			frd := (ir >> 7) & 0x1f
			funct3 := (ir >> 12) & 0x7
			frs1_id := (ir >> 15) & 0x1f
			frs2_id := (ir >> 20) & 0x1f
			funct7 := ir >> 25

			rdid = 0 // 不写入整数寄存器
			rval = 0 // 整数寄存器写入0

			// TODO: 根据fcsr中的rm字段处理舍入模式。目前使用Go默认的 "round-to-nearest, ties-to-even"。
			// 异常标志也没有完全实现。

			f1 := math.Float32frombits(vmst.Core.Fregs[frs1_id])
			f2 := math.Float32frombits(vmst.Core.Fregs[frs2_id])
			var fdest float32
			write_fdest := true

			switch funct7 {
			case FUNCT7_FADD_S: // FADD.S
				fdest = f1 + f2
			case FUNCT7_FSUB_S: // FSUB.S
				fdest = f1 - f2
			case FUNCT7_FMUL_S: // FMUL.S
				fdest = f1 * f2
			case FUNCT7_FDIV_S: // FDIV.S
				fdest = f1 / f2
			case FUNCT7_FSQRT_S: // FSQRT.S
				if frs2_id == 0 {
					fdest = float32(math.Sqrt(float64(f1)))
				} else {
					trap, write_fdest = CAUSE_ILLEGAL_INSTRUCTION, false
				}
			case FUNCT7_FSGNJ_S: // FSGNJ.S, FSGNJN.S, FSGNJX.S
				f1_bits := math.Float32bits(f1)
				f2_bits := math.Float32bits(f2)
				sign2 := f2_bits & 0x80000000
				var result_bits uint32
				switch funct3 {
				case FUNCT3_FSGNJ_S:
					result_bits = (f1_bits & 0x7fffffff) | sign2
				case FUNCT3_FSGNJN_S:
					result_bits = (f1_bits & 0x7fffffff) | (sign2 ^ 0x80000000)
				case FUNCT3_FSGNJX_S:
					result_bits = (f1_bits & 0x7fffffff) | (math.Float32bits(f1)^f2_bits)&0x80000000
				default:
					trap, write_fdest = CAUSE_ILLEGAL_INSTRUCTION, false
				}
				if write_fdest {
					fdest = math.Float32frombits(result_bits)
				}
			case FUNCT7_FMIN_MAX_S: // FMIN.S, FMAX.S
				switch funct3 {
				case FUNCT3_FMIN_S:
					if f1 < f2 || (math.IsNaN(float64(f1)) && !math.IsNaN(float64(f2))) {
						fdest = f1
					} else {
						fdest = f2
					}
				case FUNCT3_FMAX_S:
					if f1 > f2 || (math.IsNaN(float64(f1)) && !math.IsNaN(float64(f2))) {
						fdest = f1
					} else {
						fdest = f2
					}
				default:
					trap, write_fdest = CAUSE_ILLEGAL_INSTRUCTION, false
				}
			case FUNCT7_FCVT_W_S: // FCVT.W.S, FCVT.WU.S
				write_fdest = false
				rdid = frd // 这些指令写入整数寄存器
				rm := (vmst.Core.Fcsr >> 5) & 7
				// 这是一个简化的舍入实现
				var val int64
				switch rm {
				case 0: // RNE (四舍五入到最近的偶数)
					val = int64(math.RoundToEven(float64(f1)))
				case 1: // RTZ (向零舍入)
					val = int64(f1)
				case 2: // RDN (向下舍入)
					val = int64(math.Floor(float64(f1)))
				case 3: // RUP (向上舍入)
					val = int64(math.Ceil(float64(f1)))
				default: // RMM (四舍五入到最大幅度的值) - 非标准，作为兜底
					val = int64(math.Round(float64(f1)))
				}
				switch frs2_id {
				case FRS2_FCVT_W_S: // FCVT.W.S (浮点转有符号整数)
					if val > math.MaxInt32 {
						vmst.Core.Fcsr |= 0x1 /* NV */
						rval = math.MaxInt32
					} else if val < math.MinInt32 {
						vmst.Core.Fcsr |= 0x1 /* NV */
						rval = 0x80000000
					} else {
						rval = uint32(int32(val))
					}
				case FRS2_FCVT_WU_S: // FCVT.WU.S (浮点转无符号整数)
					if val < 0 {
						vmst.Core.Fcsr |= 0x1 /* NV */
						rval = 0
					} else if uint64(val) > math.MaxUint32 {
						vmst.Core.Fcsr |= 0x1 /* NV */
						rval = math.MaxUint32
					} else {
						rval = uint32(val)
					}
				default:
					trap = CAUSE_ILLEGAL_INSTRUCTION
				}
			case FUNCT7_FCVT_S_W: // FCVT.S.W, FCVT.S.WU
				rs1_val := vmst.Core.Regs[frs1_id]
				switch frs2_id {
				case FRS2_FCVT_S_W: // FCVT.S.W (有符号整数转浮点)
					fdest = float32(int32(rs1_val))
				case FRS2_FCVT_S_WU: // FCVT.S.WU (无符号整数转浮点)
					fdest = float32(rs1_val)
				default:
					trap, write_fdest = CAUSE_ILLEGAL_INSTRUCTION, false
				}
			case FUNCT7_FMV_X_W_FCLASS_S: // FMV.X.W, FCLASS.S
				write_fdest = false
				switch funct3 {
				case FUNCT3_FMV_X_W: // FMV.X.W (移动浮点寄存器到整数寄存器)
					rdid = frd
					rval = vmst.Core.Fregs[frs1_id]
				case FUNCT3_FCLASS_S: // FCLASS.S (浮点数分类)
					rdid = frd
					f1_bits := math.Float32bits(f1)
					is_neg := (f1_bits >> 31) != 0
					exp := (f1_bits >> 23) & 0xFF
					mant := f1_bits & 0x7FFFFF
					if exp == 0 && mant == 0 { // 零
						if is_neg {
							rval = 1 << 3
						} else {
							rval = 1 << 4
						}
					} else if exp == 0xFF && mant == 0 { // 无穷大
						if is_neg {
							rval = 1 << 0
						} else {
							rval = 1 << 7
						}
					} else if exp == 0xFF && mant != 0 { // NaN
						if (mant >> 22) != 0 {
							rval = 1 << 9
						} else {
							rval = 1 << 8
						} // 信号 NaN 或安静 NaN
					} else if exp == 0 { // 非规格化数
						if is_neg {
							rval = 1 << 2
						} else {
							rval = 1 << 5
						}
					} else { // 规格化数
						if is_neg {
							rval = 1 << 1
						} else {
							rval = 1 << 6
						}
					}
				default:
					trap = CAUSE_ILLEGAL_INSTRUCTION
				}
			case FUNCT7_FMV_W_X: // FMV.W.X (移动整数寄存器到浮点寄存器)
				fdest = math.Float32frombits(vmst.Core.Regs[frs1_id])
			case FUNCT7_FEQ_FLT_FLE_S: // FEQ.S, FLT.S, FLE.S
				write_fdest = false
				rdid = frd
				switch funct3 {
				case FUNCT3_FEQ_S:
					if f1 == f2 {
						rval = 1
					}
				case FUNCT3_FLT_S:
					if f1 < f2 {
						rval = 1
					}
				case FUNCT3_FLE_S:
					if f1 <= f2 {
						rval = 1
					}
				default:
					trap = CAUSE_ILLEGAL_INSTRUCTION
				}
				// TODO: 在遇到 NaN 时设置无效标志
			default:
				trap, write_fdest = CAUSE_ILLEGAL_INSTRUCTION, false
			}

			if trap == 0 && write_fdest {
				vmst.Core.Fregs[frd] = math.Float32bits(fdest)
			}
		case OPCODE_VECTOR: // 向量指令
			trap = vmst.handleVectorInstruction(ir)
		case OPCODE_FENCE: // FENCE
			rdid = 0 // 我们在这个实现中忽略 FENCE 指令
		case OPCODE_SYSTEM: // Zifencei+Zicsr
			csrno := ir >> 20
			microop := (ir >> 12) & 0x7

			if (microop & 3) != 0 { // Zicsr
				rs1imm := (ir >> 15) & 0x1f
				rs1 := vmst.Core.Regs[rs1imm]
				var writeval uint32

				switch csrno {
				case CSR_MSCRATCH:
					rval = vmst.Core.Mscratch
				case CSR_MTVEC:
					rval = vmst.Core.Mtvec
				case CSR_MIE:
					rval = vmst.Core.Mie
				case CSR_MIP:
					rval = vmst.Core.Mip
				case CSR_MEPC:
					rval = vmst.Core.Mepc
				case CSR_MSTATUS:
					rval = vmst.Core.Mstatus
				case CSR_MCAUSE:
					rval = vmst.Core.Mcause
				case CSR_MTVAL:
					rval = vmst.Core.Mtval
				case CSR_MVENDORID:
					rval = 0xff0ff0ff // mvendorid
				case CSR_MISA:
					rval = 0x40601121 // misa (XLEN=32, IMAFV+X)
				case CSR_FFLAGS:
					rval = vmst.Core.Fcsr & 0x1F
				case CSR_FRM:
					rval = (vmst.Core.Fcsr >> 5) & 0x7
				case CSR_FCSR:
					rval = vmst.Core.Fcsr
				// Vector CSRs
				case CSR_VL:
					rval = vmst.Core.Vl
				case CSR_VTYPE:
					rval = vmst.Core.Vtype
				case CSR_VLENB:
					rval = 16 // VLEN = 128 bits = 16 bytes
				case CSR_VSTART:
					rval = vmst.Core.Vstart
				default:
					// 其他 CSRs 没有实现
				}

				if (microop & 4) != 0 { // 立即数形式
					rs1 = rs1imm
				}

				switch microop & 3 {
				case FUNCT3_CSRRW:
					writeval = rs1
				case FUNCT3_CSRRS:
					writeval = rval | rs1
				case FUNCT3_CSRRC:
					writeval = rval &^ rs1
				}

				switch csrno {
				case CSR_MSCRATCH:
					vmst.Core.Mscratch = writeval
				case CSR_MTVEC:
					vmst.Core.Mtvec = writeval
				case CSR_MIE:
					vmst.Core.Mie = writeval
				case CSR_MIP:
					vmst.Core.Mip = writeval
				case CSR_MEPC:
					vmst.Core.Mepc = writeval
				case CSR_MSTATUS:
					vmst.Core.Mstatus = writeval
				case CSR_MCAUSE:
					vmst.Core.Mcause = writeval
				case CSR_MTVAL:
					vmst.Core.Mtval = writeval
				case CSR_FFLAGS:
					vmst.Core.Fcsr = (vmst.Core.Fcsr &^ 0x1F) | (writeval & 0x1F)
				case CSR_FRM:
					vmst.Core.Fcsr = (vmst.Core.Fcsr &^ 0xE0) | ((writeval & 0x7) << 5)
				case CSR_FCSR:
					vmst.Core.Fcsr = writeval
				case CSR_VSTART: // vstart CSR
					vmst.Core.Vstart = writeval
				}

			} else if microop == FUNCT3_SYSTEM_ECALL_EBREAK { // ECALL/EBREAK
				rdid = 0
				if csrno == 0 { // ECALL
					if (vmst.Core.Extraflags & 3) != 0 {
						trap = CAUSE_ECALL_FROM_M_MODE
					} else {
						trap = CAUSE_ECALL_FROM_U_MODE
					}
				} else {
					trap = CAUSE_ILLEGAL_INSTRUCTION
				}
			} else {
				trap = CAUSE_ILLEGAL_INSTRUCTION
			}

		default:
			trap = CAUSE_ILLEGAL_INSTRUCTION
		}

		if trap != 0 {
			break
		}

		if rdid != 0 {
			vmst.Core.Regs[rdid] = rval
		}

		pc += 4
	}

	vmst.Core.PC = pc

	return trap
}

// get_velement_addr 计算向量寄存器文件中特定元素的字节地址。
// 这个函数对于抽象向量寄存器的布局至关重要，
// 尤其是在处理大于1的LMUL（向量寄存器分组）和不同的SEW（标准元素宽度）时。
//
// 参数:
//
//	reg_start_idx: 向量操作中使用的起始向量寄存器的索引。
//	element_idx:   向量中当前元素的逻辑索引 (从0到vl-1)。
//	sew_bytes:     当前SEW（标准元素宽度）的字节数。
//
// 返回:
//
//	元素在 `vmst.Core.Vregs` 数组中的字节偏移量。
func (vmst *VmState) get_velement_addr(reg_start_idx uint32, element_idx uint32, sew_bytes uint32) uint32 {
	const VLEN_BYTES = 16 // VLEN (向量长度) 固定为128位（16字节）。

	// 计算每个128位物理寄存器可以容纳多少个元素。
	elements_per_reg := VLEN_BYTES / sew_bytes

	// 根据元素索引确定它属于哪个物理寄存器（相对于起始寄存器）
	// 以及它在那个寄存器中的偏移量。
	reg_offset := element_idx / elements_per_reg
	element_offset_in_reg := (element_idx % elements_per_reg) * sew_bytes

	// 计算元素的最终地址。
	actual_reg_idx := reg_start_idx + reg_offset
	addr := actual_reg_idx*VLEN_BYTES + element_offset_in_reg

	return addr
}

// handleOPIVV 处理OPIVV类型的向量指令（向量-向量整数运算）。
// 这些指令对两个向量寄存器（vs1, vs2）中的元素执行操作，并将结果写入目标向量寄存器（vd）。
//
// 参数:
//
//	ir: 编码的指令。
//
// 返回:
//
//	如果成功则返回0，如果发生非法指令陷阱则返回非零值。
func (vmst *VmState) handleOPIVV(ir uint32) int32 {
	// 从指令中解码字段
	funct6 := ir >> 26
	vm := (ir >> 25) & 1
	vd := (ir >> 7) & 0x1f
	vs1 := (ir >> 15) & 0x1f
	vs2 := (ir >> 20) & 0x1f

	// 从vtype CSR确定SEW
	sew_val := (vmst.Core.Vtype >> 2) & 0x7
	sew_bytes := uint32(1 << sew_val)

	if sew_bytes > 4 { // 当前实现只支持SEW <= 32位
		return CAUSE_ILLEGAL_INSTRUCTION
	}

	// 遍历由vl指定的元素
	for i := vmst.Core.Vstart; i < vmst.Core.Vl; i++ {
		// 如果vm=0（非屏蔽），检查v0中的屏蔽位
		if vm == 0 {
			mask_byte_index := i / 8
			mask_bit_index := i % 8
			// v0是屏蔽寄存器。检查相应的位。
			if (vmst.Core.Vregs[mask_byte_index] & (1 << mask_bit_index)) == 0 {
				continue // 如果被屏蔽，则跳过此元素
			}
		}

		// 获取操作数和目标元素的地址
		addr1 := vmst.get_velement_addr(vs1, i, sew_bytes)
		addr2 := vmst.get_velement_addr(vs2, i, sew_bytes)
		addr_dest := vmst.get_velement_addr(vd, i, sew_bytes)

		var op1, op2, result uint32

		switch funct6 {
		case FUNCT6_VADD:
			switch sew_bytes {
			case 1:
				op1 = uint32(vmst.Core.Vregs[addr1])
				op2 = uint32(vmst.Core.Vregs[addr2])
				result = op1 + op2
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr1:]))
				op2 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				result = op1 + op2
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr1:])
				op2 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				result = op1 + op2
				binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
			}
		case FUNCT6_VSUB:
			switch sew_bytes {
			case 1:
				op1 = uint32(vmst.Core.Vregs[addr1])
				op2 = uint32(vmst.Core.Vregs[addr2])
				result = op1 - op2
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr1:]))
				op2 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				result = op1 - op2
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr1:])
				op2 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				result = op1 - op2
				binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
			}
		case FUNCT6_VAND:
			switch sew_bytes {
			case 1:
				op1 = uint32(vmst.Core.Vregs[addr1])
				op2 = uint32(vmst.Core.Vregs[addr2])
				result = op1 & op2
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr1:]))
				op2 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				result = op1 & op2
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr1:])
				op2 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				result = op1 & op2
				binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
			}
		case FUNCT6_VOR:
			switch sew_bytes {
			case 1:
				op1 = uint32(vmst.Core.Vregs[addr1])
				op2 = uint32(vmst.Core.Vregs[addr2])
				result = op1 | op2
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr1:]))
				op2 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				result = op1 | op2
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr1:])
				op2 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				result = op1 | op2
				binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
			}
		case FUNCT6_VXOR:
			switch sew_bytes {
			case 1:
				op1 = uint32(vmst.Core.Vregs[addr1])
				op2 = uint32(vmst.Core.Vregs[addr2])
				result = op1 ^ op2
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr1:]))
				op2 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				result = op1 ^ op2
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr1:])
				op2 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				result = op1 ^ op2
				binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
			}
		case FUNCT6_VSLL:
			switch sew_bytes {
			case 1:
				op1 = uint32(vmst.Core.Vregs[addr1])
				op2 = uint32(vmst.Core.Vregs[addr2])
				result = uint32(byte(op2) << (op1 & 0x7))
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr1:]))
				op2 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				result = uint32(uint16(op2) << (op1 & 0xF))
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr1:])
				op2 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				result = op2 << (op1 & 0x1F)
				binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
			}
		case FUNCT6_VSRL:
			switch sew_bytes {
			case 1:
				op1 = uint32(vmst.Core.Vregs[addr1])
				op2 = uint32(vmst.Core.Vregs[addr2])
				result = uint32(byte(op2) >> (op1 & 0x7))
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr1:]))
				op2 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				result = uint32(uint16(op2) >> (op1 & 0xF))
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr1:])
				op2 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				result = op2 >> (op1 & 0x1F)
				binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
			}
		case FUNCT6_VSRA:
			switch sew_bytes {
			case 1:
				op1 = uint32(vmst.Core.Vregs[addr1])
				op2 = uint32(vmst.Core.Vregs[addr2])
				result = uint32(int8(op2) >> (op1 & 0x7))
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr1:]))
				op2 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				result = uint32(int16(op2) >> (op1 & 0xF))
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr1:])
				op2 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				result = uint32(int32(op2) >> (op1 & 0x1F))
				binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
			}
		default:
			return CAUSE_ILLEGAL_INSTRUCTION
		}
	}

	vmst.Core.Vstart = 0 // 完成后，vstart 被重置为 0
	return 0
}

// handleVectorInstruction 是向量指令的顶层分派函数。
// 它解码funct3字段，以确定要执行的向量操作的类型。
//
// 参数:
//
//	ir: 编码的指令。
//
// 返回:
//
//	一个陷阱代码，如果成功则为0，否则为非零值。
func (vmst *VmState) handleVectorInstruction(ir uint32) int32 {
	funct3 := (ir >> 12) & 0x7

	// 根据RISC-V向量规范，如果vl为0，大多数向量指令都是非法的，
	// VSETVL*指令除外。
	if funct3 != FUNCT3_OP_V && vmst.Core.Vl == 0 {
		return CAUSE_ILLEGAL_INSTRUCTION
	}

	// 根据funct3分派到相应的处理函数
	switch funct3 {
	case FUNCT3_OPIVV: // 整数 向量-向量 (Vector-Vector)
		return vmst.handleOPIVV(ir)
	case 1: // 浮点 向量-向量 (OPFVV)
		return vmst.handleVFPOPIVV(ir)
	case FUNCT3_OPIVI: // 整数 向量-立即数 (Vector-Immediate)
		return vmst.handleOPIVI(ir)
	case FUNCT3_OPIVX: // 整数 向量-标量 (Vector-Scalar)
		return vmst.handleOPIVX(ir)
	case 5: // 浮点 向量-标量 (OPFVF)
		return vmst.handleVFPOPIVF(ir)
	case FUNCT3_OP_V: // VSETVL* 指令
		return vmst.handleVSETVL(ir)
	default:
		return CAUSE_ILLEGAL_INSTRUCTION
	}
}
