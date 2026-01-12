package vm

//
// 系统调用常量
//

const (
	// VmSysCallHalt 停止虚拟机
	VmSysCallHalt = 0x1000000
	// VmSysCallYield 主动让出 CPU
	VmSysCallYield = 0x1000001
	// VmSysyCallStackProTect 栈保护
	VmSysyCallStackProTect = 0x1000002
	// 用户系统调用从这里开始
)

//
// 内存布局常量
//

const (
	// VmMemoRySize 内存大小
	VmMemoRySize = 1024 * 64 // 64KB
	// VmRamImageOffSet RAM 镜像偏移
	VmRamImageOffSet = 0x80000000
	// VmEetRamBase 扩展 RAM 基址
	VmEetRamBase = 0x10000000
)

const (
	// VmErrNone 无错误
	VmErrNone VmErr = iota
	// VmErrNotrEady 虚拟机未准备好
	VmErrNotrEady
	// VmErrMemRd 内存读取错误
	VmErrMemRd
	// VmErrMemWr 内存写入错误
	VmErrMemWr
	// VmErrBadSysCall 无效的系统调用
	VmErrBadSysCall
	// VmErrHung 虚拟机挂起
	VmErrHung
	// VmErrIntErnalCore 内部核心错误
	VmErrIntErnalCore
	// VmErrIntErnalState 内部状态错误
	VmErrIntErnalState
	// VmErrArgs 参数错误
	VmErrArgs
)

const (
	// VmEvtTypErr 错误事件
	VmEvtTypErr VmEvtTyp = iota
	// VmEvtTypSysCall 系统调用事件
	VmEvtTypSysCall
	// VmEvtTypEnd 虚拟机结束事件
	VmEvtTypEnd
)

const (
	// Arg0 第一个参数
	Arg0 VmArg = iota
	// Arg1 第二个参数
	Arg1
	// Ret 返回值
	Ret
)

const (
	// VmStatusPaused 已暂停
	VmStatusPaused VmStatus = iota
	// VmStatusRunnIng 运行中
	VmStatusRunnIng
	// VmStatusError 出错
	VmStatusError
	// VmStatusEnded 已结束
	VmStatusEnded
)

// Machine-Level Trap Causes
const (
	CAUSE_INSTRUCTION_ADDRESS_MISALIGNED = 0 // 指令地址未对齐
	CAUSE_INSTRUCTION_ACCESS_FAULT       = 1 // 指令访问故障
	CAUSE_ILLEGAL_INSTRUCTION            = 2 // 非法指令
	CAUSE_BREAKPOINT                     = 3 // 断点
	CAUSE_LOAD_ADDRESS_MISALIGNED        = 4 // 加载地址未对齐
	CAUSE_LOAD_ACCESS_FAULT              = 5 // 加载访问故障
	CAUSE_STORE_AMO_ADDRESS_MISALIGNED   = 6 // 存储/AMO地址未对齐
	CAUSE_STORE_AMO_ACCESS_FAULT         = 7 // 存储/AMO访问故障
	CAUSE_ECALL_FROM_U_MODE              = 8 // 来自U模式的ECALL
	CAUSE_ECALL_FROM_S_MODE              = 9 // 来自S模式的ECALL
	// CAUSE 10 is reserved
	CAUSE_ECALL_FROM_M_MODE = 11 // 来自M模式的ECALL
)

// CSR 地址
const (
	CSR_FFLAGS    = 0x001 // 浮点异常标志
	CSR_FRM       = 0x002 // 浮点舍入模式
	CSR_FCSR      = 0x003 // 浮点控制和状态寄存器
	CSR_VSTART    = 0x008 // 向量起始元素索引
	CSR_MSTATUS   = 0x300 // 机器状态寄存器
	CSR_MISA      = 0x301 // 机器 ISA 和扩展
	CSR_MIE       = 0x304 // 机器中断使能
	CSR_MTVEC     = 0x305 // 机器陷阱处理程序基地址
	CSR_MSCRATCH  = 0x340 // 机器陷阱处理程序的暂存寄存器
	CSR_MEPC      = 0x341 // 机器异常程序计数器
	CSR_MCAUSE    = 0x342 // 机器陷阱原因
	CSR_MTVAL     = 0x343 // 机器陷阱值
	CSR_MIP       = 0x344 // 机器中断挂起
	CSR_MVENDORID = 0xF11 // 供应商 ID
	CSR_VL        = 0xC20 // 向量长度
	CSR_VTYPE     = 0xC21 // 向量数据类型
	CSR_VLENB     = 0xC22 // 向量寄存器长度 (以字节为单位)
)

// VTYPE 的 vlmul 编码
const (
	// 注意：这些是 vtype 寄存器中 vlmul 字段的值。
	VLMUL_1 = 0b000
	VLMUL_2 = 0b001
	VLMUL_4 = 0b010
	VLMUL_8 = 0b011
)

// RISC-V 操作码
const (
	OPCODE_LUI      = 0x37 // LUI (load upper immediate) 将立即数 imm 装入 rd 的高 20 位，低 12 位补 0
	OPCODE_AUIPC    = 0x17 // AUIPC (add upper immediate to pc) 用于生成 pc 相关地址，结果为 pc + imm，imm 是 20 位立即数
	OPCODE_JAL      = 0x6F // JAL (jump and link) 跳转到 pc + imm，并将下一条指令的地址 (pc + 4) 保存到 rd
	OPCODE_JALR     = 0x67 // JALR (jump and link register) 跳转到 rs1 + imm，并将下一条指令的地址 (pc + 4) 保存到 rd
	OPCODE_BRANCH   = 0x63 // BRANCH 包含条件跳转指令
	OPCODE_LOAD     = 0x03 // LOAD 包含加载指令
	OPCODE_STORE    = 0x23 // STORE 包含存储指令
	OPCODE_OP_IMM   = 0x13 // OP_IMM 包含立即数算术和逻辑指令
	OPCODE_OP       = 0x33 // OP 包含寄存器-寄存器算术和逻辑指令
	OPCODE_FENCE    = 0x0F // FENCE 用于对内存和 I/O 操作进行排序
	OPCODE_SYSTEM   = 0x73 // SYSTEM 包含系统调用和 CSR 指令
	OPCODE_LOAD_FP  = 0x07 // LOAD_FP 包含浮点加载指令
	OPCODE_STORE_FP = 0x27 // STORE_FP 包含浮点存储指令
	OPCODE_OP_FP    = 0x53 // OP_FP 包含浮点算术指令
	OPCODE_VECTOR   = 0x57 // VECTOR 包含向量指令
)

// 用于 BRANCH 的 FUNCT3
const (
	FUNCT3_BEQ  = 0 // BEQ (branch if equal) 如果 rs1 == rs2 则跳转
	FUNCT3_BNE  = 1 // BNE (branch if not equal) 如果 rs1 != rs2 则跳转
	FUNCT3_BLT  = 4 // BLT (branch if less than) 如果 rs1 < rs2 则跳转 (有符号)
	FUNCT3_BGE  = 5 // BGE (branch if greater than or equal) 如果 rs1 >= rs2 则跳转 (有符号)
	FUNCT3_BLTU = 6 // BLTU (branch if less than, unsigned) 如果 rs1 < rs2 则跳转 (无符号)
	FUNCT3_BGEU = 7 // BGEU (branch if greater than or equal, unsigned) 如果 rs1 >= rs2 则跳转 (无符号)
)

// 用于 LOAD 的 FUNCT3
const (
	FUNCT3_LB  = 0 // LB (load byte) 加载一个字节并进行符号扩展
	FUNCT3_LH  = 1 // LH (load halfword) 加载一个半字并进行符号扩展
	FUNCT3_LW  = 2 // LW (load word) 加载一个字
	FUNCT3_LBU = 4 // LBU (load byte, unsigned) 加载一个字节并进行零扩展
	FUNCT3_LHU = 5 // LHU (load halfword, unsigned) 加载一个半字并进行零扩展
)

// 用于 STORE 的 FUNCT3
const (
	FUNCT3_SB = 0 // SB (store byte) 存储一个字节
	FUNCT3_SH = 1 // SH (store halfword) 存储一个半字
	FUNCT3_SW = 2 // SW (store word) 存储一个字
)

// 用于 OP_IMM 和 OP 的 FUNCT3
const (
	FUNCT3_ADD_SUB = 0 // ADD/SUB/ADDI
	FUNCT3_SLL     = 1 // SLL/SLLI (shift left logical)
	FUNCT3_SLT     = 2 // SLT/SLTI (set less than)
	FUNCT3_SLTU    = 3 // SLTU/SLTIU (set less than, unsigned)
	FUNCT3_XOR     = 4 // XOR/XORI
	FUNCT3_SRL_SRA = 5 // SRL/SRA/SRLI/SRAI (shift right logical/arithmetic)
	FUNCT3_OR      = 6 // OR/ORI
	FUNCT3_AND     = 7 // AND/ANDI
)

// 用于 OP 的 FUNCT7
const (
	FUNCT7_SUB = 0x20 // SUB (subtract)
	FUNCT7_SRA = 0x20 // SRA (shift right arithmetic)
)

// 用于 M 扩展的 FUNCT7
const (
	FUNCT7_M = 1
)

// 用于 M 扩展的 FUNCT3
const (
	FUNCT3_MUL    = 0 // MUL (multiply)
	FUNCT3_MULH   = 1 // MULH (multiply high, signed)
	FUNCT3_MULHSU = 2 // MULHSU (multiply high, signed/unsigned)
	FUNCT3_MULHU  = 3 // MULHU (multiply high, unsigned)
	FUNCT3_DIV    = 4 // DIV (divide, signed)
	FUNCT3_DIVU   = 5 // DIVU (divide, unsigned)
	FUNCT3_REM    = 6 // REM (remainder, signed)
	FUNCT3_REMU   = 7 // REMU (remainder, unsigned)
)

// 用于 F 扩展 LOAD/STORE 的 FUNCT3
const (
	FUNCT3_FLW = 2 // FLW (floating-point load word)
	FUNCT3_FSW = 2 // FSW (floating-point store word)
)

// 用于 F 扩展 OP_FP 的 FUNCT7
const (
	FUNCT7_FADD_S           = 0b0000000 // FADD.S (floating-point add, single-precision)
	FUNCT7_FSUB_S           = 0b0000100 // FSUB.S (floating-point subtract, single-precision)
	FUNCT7_FMUL_S           = 0b0001000 // FMUL.S (floating-point multiply, single-precision)
	FUNCT7_FDIV_S           = 0b0001100 // FDIV.S (floating-point divide, single-precision)
	FUNCT7_FSQRT_S          = 0b0101100 // FSQRT.S (floating-point square root, single-precision)
	FUNCT7_FSGNJ_S          = 0b0010000 // FSGNJ.S (floating-point sign injection, single-precision)
	FUNCT7_FMIN_MAX_S       = 0b0010100 // FMIN.S/FMAX.S (floating-point minimum/maximum, single-precision)
	FUNCT7_FCVT_W_S         = 0b1100000 // FCVT.W.S/FCVT.WU.S (convert floating-point to integer, single-precision)
	FUNCT7_FCVT_S_W         = 0b1101000 // FCVT.S.W/FCVT.S.WU (convert integer to floating-point, single-precision)
	FUNCT7_FMV_X_W_FCLASS_S = 0b1110000 // FMV.X.W/FCLASS.S (move float to integer register/classify float)
	FUNCT7_FEQ_FLT_FLE_S    = 0b1010000 // FEQ.S/FLT.S/FLE.S (floating-point compare, single-precision)
	FUNCT7_FMV_W_X          = 0b1111000 // FMV.W.X (move integer to float register)
)

// 用于 FSGNJ.S 的 FUNCT3
const (
	FUNCT3_FSGNJ_S  = 0 // FSGNJ.S (sign injection)
	FUNCT3_FSGNJN_S = 1 // FSGNJN.S (sign injection negated)
	FUNCT3_FSGNJX_S = 2 // FSGNJX.S (sign injection XOR)
)

// 用于 FMIN/FMAX.S 的 FUNCT3
const (
	FUNCT3_FMIN_S = 0 // FMIN.S (minimum)
	FUNCT3_FMAX_S = 1 // FMAX.S (maximum)
)

// 用于 FEQ/FLT/FLE.S 的 FUNCT3
const (
	FUNCT3_FLE_S = 0 // FLE.S (less than or equal)
	FUNCT3_FLT_S = 1 // FLT.S (less than)
	FUNCT3_FEQ_S = 2 // FEQ.S (equal)
)

// 用于 FMV_X_W/FCLASS.S 的 FUNCT3
const (
	FUNCT3_FMV_X_W  = 0 // FMV.X.W (move float to integer register)
	FUNCT3_FCLASS_S = 1 // FCLASS.S (classify float)
)

// 用于 FCVT.S.W/WU 的 rs2 值
const (
	FRS2_FCVT_S_W  = 0 // FCVT.S.W (convert word to float)
	FRS2_FCVT_S_WU = 1 // FCVT.S.WU (convert word unsigned to float)
)

// 用于 FCVT.W/WU.S 的 rs2 值
const (
	FRS2_FCVT_W_S  = 0 // FCVT.W.S (convert float to word)
	FRS2_FCVT_WU_S = 1 // FCVT.WU.S (convert float to word unsigned)
)

// 用于 V 扩展的 FUNCT3
const (
	FUNCT3_OPIVV = 0b000
	FUNCT3_OPIVI = 0b011
	FUNCT3_OPIVX = 0b110
	FUNCT3_OP_V  = 7
)

// 用于 OPIVV 指令的 Funct6 操作码
const (
	FUNCT6_VADD  = 0b000000 // VADD (vector add)
	FUNCT6_VSUB  = 0b000010 // VSUB (vector subtract)
	FUNCT6_VRSUB = 0b000011 // VRSUB (vector reverse subtract)
	FUNCT6_VAND  = 0b001001 // VAND (vector and)
	FUNCT6_VOR   = 0b001010 // VOR (vector or)
	FUNCT6_VXOR  = 0b001011 // VXOR (vector xor)
	FUNCT6_VSLL  = 0b001101 // VSLL (vector shift left logical)
	FUNCT6_VSRL  = 0b001110 // VSRL (vector shift right logical)
	FUNCT6_VSRA  = 0b001111 // VSRA (vector shift right arithmetic)
)

// 用于 OPFVV 和 OPFVF 指令的 Funct6 操作码
const (
	FUNCT6_VFADD  = 0b100000 // VFADD (vector float add)
	FUNCT6_VFSUB  = 0b100010 // VFSUB (vector float subtract)
	FUNCT6_VFRSUB = 0b100011 // VFRSUB (vector float reverse subtract)
	FUNCT6_VFMUL  = 0b100101 // VFMUL (vector float multiply)
	FUNCT6_VFRDIV = 0b100110 // VFRDIV (vector float reverse divide)
	FUNCT6_VFDIV  = 0b100111 // VFDIV (vector float divide)
)

// 用于 SYSTEM 的 FUNCT3
const (
	FUNCT3_SYSTEM_ECALL_EBREAK = 0 // ECALL/EBREAK
	FUNCT3_CSRRW               = 1 // CSRRW (atomic read/write CSR)
	FUNCT3_CSRRS               = 2 // CSRRS (atomic read and set bits in CSR)
	FUNCT3_CSRRC               = 3 // CSRRC (atomic read and clear bits in CSR)
	FUNCT3_CSRRWI              = 5 // 未实现
	FUNCT3_CSRRSI              = 6 // 未实现
	FUNCT3_CSRRCI              = 7 // 未实现
)
