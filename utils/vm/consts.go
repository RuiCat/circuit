package vm

// 这个文件定义了虚拟机（VM）使用的各种常量。
// 这些常量包括系统调用代码、内存布局、错误码、事件类型、
// RISC-V指令操作码、功能码（funct3/funct7）、CSR地址等。
// 将这些值定义为常量可以提高代码的可读性和可维护性。

// --- 系统调用常量 ---
// 定义了VM支持的特殊系统调用代码。
const (
	VmSysCallHalt          = 0x1000000 // 停止虚拟机执行。
	VmSysCallYield         = 0x1000001 // 主动让出CPU，用于协作式多任务。
	VmSysyCallStackProTect = 0x1000002 // 栈保护。
	// 用户自定义的系统调用可以从这里开始。
)

// --- 内存布局常量 ---
// 定义了VM的内存模型和地址空间。
const (
	VmMemoRySize     = 1024 * 64  // 主内存大小，这里设置为 64KB。
	VmRamImageOffSet = 0x80000000 // RAM 在地址空间中的起始偏移。加载的程序镜像将从这里开始。
	VmEetRamBase     = 0x10000000 // 扩展RAM的基地址（如果支持）。
)

// --- VM 错误码 ---
// VmErr 定义了VM在运行过程中可能遇到的各种错误类型。
const (
	VmErrNone          VmErr = iota // 0: 无错误。
	VmErrNotrEady                   // 1: 虚拟机尚未准备好执行。
	VmErrMemRd                      // 2: 内存读取错误（例如，地址越界）。
	VmErrMemWr                      // 3: 内存写入错误。
	VmErrBadSysCall                 // 4: 无效的系统调用代码。
	VmErrHung                       // 5: 虚拟机挂起（例如，陷入死循环）。
	VmErrIntErnalCore               // 6: 内部核心逻辑错误。
	VmErrIntErnalState              // 7: 内部状态机错误。
	VmErrArgs                       // 8: 传递给VM的参数错误。
)

// --- VM 事件类型 ---
// VmEvtTyp 定义了VM可以向外部报告的事件类型。
const (
	VmEvtTypErr     VmEvtTyp = iota // 0: 发生错误事件。
	VmEvtTypSysCall                 // 1: 发生系统调用事件。
	VmEvtTypEnd                     // 2: 虚拟机执行结束事件。
)

// --- VM 参数标识 ---
// VmArg 用于标识传递给或来自VM的特定参数，通常与寄存器关联。
const (
	Arg0 VmArg = iota // 0: 通常是第一个参数（如 a0 寄存器）。
	Arg1              // 1: 第二个参数（如 a1 寄存器）。
	Ret               // 2: 返回值（如 a0 寄存器）。
)

// --- VM 状态 ---
// VmStatus 定义了虚拟机的几种可能状态。
const (
	VmStatusPaused  VmStatus = iota // 0: 已暂停。
	VmStatusRunnIng                 // 1: 正在运行。
	VmStatusError                   // 2: 处于错误状态。
	VmStatusEnded                   // 3: 执行已结束。
)

// --- 特殊陷阱代码 ---
// 定义了内部使用的特殊陷阱代码。
const (
	TRAP_CODE_EXIT = -1 // 一个特殊的内部代码，用于表示程序通过 syscall 正常退出。
)

// --- RISC-V 机器级陷阱原因 ---
// 这些常量对应于 RISC-V 规范中定义的 `mcause` 寄存器的值。
const (
	CAUSE_INSTRUCTION_ADDRESS_MISALIGNED = 0 // 异常：指令地址未对齐。
	CAUSE_INSTRUCTION_ACCESS_FAULT       = 1 // 异常：指令访问故障（例如，从无执行权限的内存区域取指）。
	CAUSE_ILLEGAL_INSTRUCTION            = 2 // 异常：遇到非法或不支持的指令。
	CAUSE_BREAKPOINT                     = 3 // 异常：执行 `EBREAK` 指令。
	CAUSE_LOAD_ADDRESS_MISALIGNED        = 4 // 异常：加载地址未对齐。
	CAUSE_LOAD_ACCESS_FAULT              = 5 // 异常：加载访问故障（例如，从无效地址加载）。
	CAUSE_STORE_ADDRESS_MISALIGNED       = 6 // 异常：存储或AMO（原子操作）地址未对齐。
	CAUSE_STORE_ACCESS_FAULT             = 7 // 异常：存储或AMO访问故障。
	CAUSE_USER_ECALL                     = 8 // 异常：在用户模式下执行 `ECALL`。
	CAUSE_ECALL_FROM_S_MODE              = 9 // 异常：在监管者模式下执行 `ECALL`。
	// CAUSE 10 is reserved
	CAUSE_ECALL_FROM_M_MODE = 11 // 异常：在机器模式下执行 `ECALL`。
)

// --- CSR (Control and Status Register) 地址 ---
// 定义了标准RISC-V特权级规范中的一些常用CSR的地址。
const (
	CSR_FFLAGS    = 0x001 // F: 浮点异常标志。
	CSR_FRM       = 0x002 // F: 浮点动态舍入模式。
	CSR_FCSR      = 0x003 // F: 浮点控制和状态寄存器（FRM + FFLAGS）。
	CSR_VSTART    = 0x008 // V: 向量操作的起始元素索引。
	CSR_MSTATUS   = 0x300 // M: 机器状态寄存器。
	CSR_MISA      = 0x301 // M: 机器ISA（指令集架构）和扩展。
	CSR_MIE       = 0x304 // M: 机器中断使能。
	CSR_MTVEC     = 0x305 // M: 机器陷阱处理程序基地址。
	CSR_MSCRATCH  = 0x340 // M: 供机器陷阱处理程序使用的暂存寄存器。
	CSR_MEPC      = 0x341 // M: 机器异常程序计数器。
	CSR_MCAUSE    = 0x342 // M: 机器陷阱原因。
	CSR_MTVAL     = 0x343 // M: 机器陷阱值（例如，出错的地址或指令）。
	CSR_MIP       = 0x344 // M: 机器中断挂起。
	CSR_MVENDORID = 0xF11 // M: 供应商ID（只读）。
	CSR_VL        = 0xC20 // V: 当前向量指令处理的元素数量。
	CSR_VTYPE     = 0xC21 // V: 向量数据类型和分组配置。
	CSR_VLENB     = 0xC22 // V: 向量寄存器长度（以字节为单位）。
)

// --- VTYPE 寄存器的 vlmul 编码 ---
// 这些值用于 `vtype` CSR，定义了向量寄存器的分组方式 (LMUL)。
const (
	VLMUL_1 = 0b000 // LMUL=1, 每个向量寄存器独立。
	VLMUL_2 = 0b001 // LMUL=2, 每2个向量寄存器被分组。
	VLMUL_4 = 0b010 // LMUL=4, 每4个向量寄存器被分组。
	VLMUL_8 = 0b011 // LMUL=8, 每8个向量寄存器被分组。
)

// --- RISC-V 主操作码 (Opcode) ---
// 这些是 RV32I 和一些标准扩展指令集中7位操作码的定义。
const (
	OPCODE_LUI      = 0x37 // U-Type: LUI (Load Upper Immediate)。
	OPCODE_AUIPC    = 0x17 // U-Type: AUIPC (Add Upper Immediate to PC)。
	OPCODE_JAL      = 0x6F // J-Type: JAL (Jump and Link)。
	OPCODE_JALR     = 0x67 // I-Type: JALR (Jump and Link Register)。
	OPCODE_BRANCH   = 0x63 // B-Type: 条件分支指令 (BEQ, BNE, etc.)。
	OPCODE_LOAD     = 0x03 // I-Type: 加载指令 (LB, LH, LW, etc.)。
	OPCODE_STORE    = 0x23 // S-Type: 存储指令 (SB, SH, SW)。
	OPCODE_OP_IMM   = 0x13 // I-Type: 立即数算术/逻辑指令 (ADDI, SLTI, etc.)。
	OPCODE_OP       = 0x33 // R-Type: 寄存器-寄存器算术/逻辑指令 (ADD, SUB, etc.)。
	OPCODE_FENCE    = 0x0F // I-Type: FENCE, 用于内存排序。
	OPCODE_SYSTEM   = 0x73 // I-Type: 系统指令 (ECALL, EBREAK, CSRs)。
	OPCODE_LOAD_FP  = 0x07 // F-Ext: 浮点加载指令 (FLW)。
	OPCODE_STORE_FP = 0x27 // F-Ext: 浮点存储指令 (FSW)。
	OPCODE_OP_FP    = 0x53 // F-Ext: 浮点计算指令。
	OPCODE_MADD     = 0x43 // F-Ext (FMA): FMADD.S/D
	OPCODE_MSUB     = 0x47 // F-Ext (FMA): FMSUB.S/D
	OPCODE_NMSUB    = 0x4B // F-Ext (FMA): FNMSUB.S/D
	OPCODE_NMADD    = 0x4F // F-Ext (FMA): FNMADD.S/D
	OPCODE_VECTOR   = 0x57 // V-Ext: 向量指令。
	OPCODE_MISC_MEM = 0x0F // 与 FENCE 相同的操作码，通过 funct3 区分。
	OPCODE_AMO      = 0x2F // A-Ext: 原子内存操作 (Atomic Memory Operations)。
)

// --- Funct3 字段常量 ---

// 用于 BRANCH 操作码
const (
	FUNCT3_BEQ  = 0 // BEQ (Branch if Equal)
	FUNCT3_BNE  = 1 // BNE (Branch if Not Equal)
	FUNCT3_BLT  = 4 // BLT (Branch if Less Than, signed)
	FUNCT3_BGE  = 5 // BGE (Branch if Greater or Equal, signed)
	FUNCT3_BLTU = 6 // BLTU (Branch if Less Than, unsigned)
	FUNCT3_BGEU = 7 // BGEU (Branch if Greater or Equal, unsigned)
)

// 用于 LOAD 操作码
const (
	FUNCT3_LB  = 0 // LB (Load Byte, signed)
	FUNCT3_LH  = 1 // LH (Load Half-word, signed)
	FUNCT3_LW  = 2 // LW (Load Word)
	FUNCT3_LBU = 4 // LBU (Load Byte, unsigned)
	FUNCT3_LHU = 5 // LHU (Load Half-word, unsigned)
)

// 用于 STORE 操作码
const (
	FUNCT3_SB = 0 // SB (Store Byte)
	FUNCT3_SH = 1 // SH (Store Half-word)
	FUNCT3_SW = 2 // SW (Store Word)
)

// 用于 OP_IMM 和 OP 操作码
const (
	FUNCT3_ADD_SUB = 0 // ADD/SUB (OP) / ADDI (OP_IMM)
	FUNCT3_SLL     = 1 // SLL (OP) / SLLI (OP_IMM)
	FUNCT3_SLT     = 2 // SLT (OP) / SLTI (OP_IMM)
	FUNCT3_SLTU    = 3 // SLTU (OP) / SLTIU (OP_IMM)
	FUNCT3_XOR     = 4 // XOR (OP) / XORI (OP_IMM)
	FUNCT3_SRL_SRA = 5 // SRL/SRA (OP) / SRLI/SRAI (OP_IMM)
	FUNCT3_OR      = 6 // OR (OP) / ORI (OP_IMM)
	FUNCT3_AND     = 7 // AND (OP) / ANDI (OP_IMM)
)

// --- Funct7 字段常量 ---

// 用于 OP 操作码
const (
	FUNCT7_SUB = 0x20 // 用于区分 ADD 和 SUB 指令。
	FUNCT7_SRA = 0x20 // 用于区分 SRL 和 SRA 指令。
)

// 用于 M-扩展 (整数乘除法)
const (
	FUNCT7_M = 1 // 所有 M 扩展的 R-Type 指令都有这个 funct7。
)

// 用于 M-扩展 的 Funct3
const (
	FUNCT3_MUL    = 0 // MUL: 乘法。
	FUNCT3_MULH   = 1 // MULH: 有符号乘法高位。
	FUNCT3_MULHSU = 2 // MULHSU: 有符号 x 无符号乘法高位。
	FUNCT3_MULHU  = 3 // MULHU: 无符号乘法高位。
	FUNCT3_DIV    = 4 // DIV: 有符号除法。
	FUNCT3_DIVU   = 5 // DIVU: 无符号除法。
	FUNCT3_REM    = 6 // REM: 有符号取余。
	FUNCT3_REMU   = 7 // REMU: 无符号取余。
)

// 用于 A-扩展 (原子指令)
const (
	FUNCT3_AMO_W = 2 // *.W for AMO (32-bit)
)
const (
	FUNCT5_LR      = 0b00010
	FUNCT5_SC      = 0b00011
	FUNCT5_AMOSWAP = 0b00001
	FUNCT5_AMOADD  = 0b00000
	FUNCT5_AMOXOR  = 0b00100
	FUNCT5_AMOAND  = 0b01100
	FUNCT5_AMOOR   = 0b01000
	FUNCT5_AMOMIN  = 0b10000
	FUNCT5_AMOMAX  = 0b10100
	FUNCT5_AMOMINU = 0b11000
	FUNCT5_AMOMAXU = 0b11100
)

// 用于 F/D-扩展 (浮点) LOAD/STORE
const (
	FUNCT3_FLW = 2 // FLW (Floating-point Load Word)
	FUNCT3_FLD = 3 // FLD (Floating-point Load Double)
	FUNCT3_FSW = 2 // FSW (Floating-point Store Word)
	FUNCT3_FSD = 3 // FSD (Floating-point Store Double)
)

// 用于 F/D-扩展 OP_FP 的 Funct7
const (
	FUNCT7_FADD_S     = 0b0000000 // FADD.S
	FUNCT7_FSUB_S     = 0b0000100 // FSUB.S
	FUNCT7_FMUL_S     = 0b0001000 // FMUL.S
	FUNCT7_FDIV_S     = 0b0001100 // FDIV.S
	FUNCT7_FSQRT_S    = 0b0101100 // FSQRT.S
	FUNCT7_FSGNJ_S    = 0b0010000 // FSGNJ.S, FSGNJN.S, FSGNJX.S
	FUNCT7_FMIN_MAX_S = 0b0010100 // FMIN.S, FMAX.S

	FUNCT7_FCVT_W_S = 0b1100000 // FCVT.W.S, FCVT.WU.S
	FUNCT7_FCVT_W_D = 0b1100001 // FCVT.W.D, FCVT.WU.D
	FUNCT7_FCVT_S_W = 0b1101000 // FCVT.S.W, FCVT.S.WU
	FUNCT7_FCVT_D_W = 0b1101001 // FCVT.D.W, FCVT.D.WU
	FUNCT7_FCVT_S_D = 0b0100000 // FCVT.S.D
	FUNCT7_FCVT_D_S = 0b0100001 // FCVT.D.S

	FUNCT7_FMV_X_W_FCLASS_S = 0b1110000 // FMV.X.W, FCLASS.S
	FUNCT7_FEQ_FLT_FLE_S    = 0b1010000 // FEQ.S, FLT.S, FLE.S
	FUNCT7_FMV_W_X          = 0b1111000 // FMV.W.X

	// D-extension funct7 have the LSB set
	FUNCT7_FADD_D        = 0b0000001 // FADD.D
	FUNCT7_FSUB_D        = 0b0000101 // FSUB.D
	FUNCT7_FMUL_D        = 0b0001001 // FMUL.D
	FUNCT7_FDIV_D        = 0b0001101 // FDIV.D
	FUNCT7_FSQRT_D       = 0b0101101 // FSQRT.D
	FUNCT7_FSGNJ_D       = 0b0010001 // FSGNJ.D, FSGNJN.D, FSGNJX.D
	FUNCT7_FMIN_MAX_D    = 0b0010101 // FMIN.D, FMAX.D
	FUNCT7_FMV_X_D       = 0b1110001 // FMV.X.D
	FUNCT7_FEQ_FLT_FLE_D = 0b1010001 // FEQ.D, FLT.D, FLE.D
	FUNCT7_FCLASS_D      = 0b1110001 // FCLASS.D
	FUNCT7_FMV_D_X       = 0b1111001 // FMV.D.X
)

// 用于 F-扩展 的 Funct3 (当 Funct7 确定大类后)
const (
	FUNCT3_FSGNJ_S  = 0 // FSGNJ.S
	FUNCT3_FSGNJN_S = 1 // FSGNJN.S
	FUNCT3_FSGNJX_S = 2 // FSGNJX.S

	FUNCT3_FMIN_S = 0 // FMIN.S
	FUNCT3_FMAX_S = 1 // FMAX.S

	FUNCT3_FLE_S = 0 // FLE.S
	FUNCT3_FLT_S = 1 // FLT.S
	FUNCT3_FEQ_S = 2 // FEQ.S

	FUNCT3_FMV_X_W  = 0 // FMV.X.W
	FUNCT3_FCLASS_S = 1 // FCLASS.S
)

// 用于 D-扩展 的 Funct3 (当 Funct7 确定大类后)
// 注意：许多 D-扩展 的 funct3 与 F-扩展 共享相同的值。
// 为清晰起见，在此处重新定义它们。
const (
	FUNCT3_FSGNJ_D  = 0 // FSGNJ.D
	FUNCT3_FSGNJN_D = 1 // FSGNJN.D
	FUNCT3_FSGNJX_D = 2 // FSGNJX.D

	FUNCT3_FMIN_D = 0 // FMIN.D
	FUNCT3_FMAX_D = 1 // FMAX.D

	FUNCT3_FLE_D = 0 // FLE.D
	FUNCT3_FLT_D = 1 // FLT.D
	FUNCT3_FEQ_D = 2 // FEQ.D

	FUNCT3_FMV_X_D  = 0 // FMV.X.D
	FUNCT3_FCLASS_D = 1 // FCLASS.D
)

// 用于 F/D-扩展转换指令的 rs2 字段编码
const (
	FRS2_FCVT_S_W  = 0 // FCVT.S.W (有符号整转单精度)
	FRS2_FCVT_S_WU = 1 // FCVT.S.WU (无符号整转单精度)

	FRS2_FCVT_W_S  = 0 // FCVT.W.S (单精度转有符号整)
	FRS2_FCVT_WU_S = 1 // FCVT.WU.S (单精度转无符号整)

	FRS2_FCVT_D_W  = 0 // FCVT.D.W (有符号整转双精度)
	FRS2_FCVT_D_WU = 1 // FCVT.D.WU (无符号整转双精度)

	FRS2_FCVT_W_D  = 0 // FCVT.W.D (双精度转有符号整)
	FRS2_FCVT_WU_D = 1 // FCVT.WU.D (双精度转无符号整)

	FRS2_FCVT_S_D = 1 // FCVT.S.D (双精度转单精度)
	FRS2_FCVT_D_S = 0 // FCVT.D.S (单精度转双精度)
)

// 用于 V-扩展 的 Funct3
const (
	FUNCT3_OPIVV = 0b000 // 向量-向量
	FUNCT3_OPIVI = 0b011 // 向量-立即数
	FUNCT3_OPIVX = 0b110 // 向量-标量
	FUNCT3_OP_V  = 7     // vsetvl/vsetvli
)

// 用于 V-扩展 OPIVV (整数) 的 Funct6
const (
	FUNCT6_VADD  = 0b000000 // VADD
	FUNCT6_VSUB  = 0b000010 // VSUB
	FUNCT6_VRSUB = 0b000011 // VRSUB
	FUNCT6_VAND  = 0b001001 // VAND
	FUNCT6_VOR   = 0b001010 // VOR
	FUNCT6_VXOR  = 0b001011 // VXOR
	FUNCT6_VSLL  = 0b001101 // VSLL
	FUNCT6_VSRL  = 0b001110 // VSRL
	FUNCT6_VSRA  = 0b001111 // VSRA
)

// 用于 V-扩展 OPFVV/OPFVF (浮点) 的 Funct6
const (
	FUNCT6_VFADD  = 0b100000 // VFADD
	FUNCT6_VFSUB  = 0b100010 // VFSUB
	FUNCT6_VFRSUB = 0b100011 // VFRSUB
	FUNCT6_VFMUL  = 0b100101 // VFMUL
	FUNCT6_VFRDIV = 0b100110 // VFRDIV
	FUNCT6_VFDIV  = 0b100111 // VFDIV
)

// 用于 SYSTEM 操作码
const (
	FUNCT3_SYSTEM_ECALL_EBREAK = 0 // ECALL/EBREAK
	FUNCT3_CSRRW               = 1 // CSRRW (Atomic Read/Write CSR)
	FUNCT3_CSRRS               = 2 // CSRRS (Atomic Read and Set bits in CSR)
	FUNCT3_CSRRC               = 3 // CSRRC (Atomic Read and Clear bits in CSR)
	FUNCT3_CSRRWI              = 5 // CSRRWI (立即数版本)
	FUNCT3_CSRRSI              = 6 // CSRRSI
	FUNCT3_CSRRCI              = 7 // CSRRCI
)
