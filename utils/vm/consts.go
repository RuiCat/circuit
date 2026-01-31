package vm

// 这个文件定义了虚拟机（VM）使用的各种常量。
// 这些常量包括系统调用代码、内存布局、错误码、事件类型、
// RISC-V指令操作码、功能码（funct3/funct7）、CSR地址等。
// 将这些值定义为常量可以提高代码的可读性和可维护性。

// --- 系统调用常量 ---
// 定义了VM支持的特殊系统调用代码。
const (
	VmSysCallHalt = 0x1000000 // 停止虚拟机执行。
	// 用户自定义的系统调用可以从这里开始。
)

const (
	// 特权级别
	PRIV_USER       = 0
	PRIV_SUPERVISOR = 1
	PRIV_MACHINE    = 3

	// 用于 MMU 的内存访问类型
	VmMemAccessInstruction = 0
	VmMemAccessLoad        = 1
	VmMemAccessStore       = 2
)

// --- 内存布局常量 ---
// 定义了VM的内存模型和地址空间。
const (
	VmRamImageOffSet = 0x80000000 // RAM 在地址空间中的起始偏移。加载的程序镜像将从这里开始。
	VmEetRamBase     = 0x10000000 // 扩展RAM的基地址（如果支持）。
)

// --- VM 错误码 ---
// VmErr 定义了VM在运行过程中可能遇到的各种错误类型。
const (
	VmErrNone         VmErr = iota // 0: 无错误。
	VmErrNotrEady                  // 1: 虚拟机尚未准备好执行。
	VmErrMemRd                     // 2: 内存读取错误（例如，地址越界）。
	VmErrMemWr                     // 3: 内存写入错误。
	VmErrBadSysCall                // 4: 无效的系统调用代码。
	VmErrHung                      // 5: 虚拟机挂起（例如，陷入死循环）。
	VmErrIntErnalCore              // 6: 内部核心逻辑错误。
	VmErrArgs                      // 7: 传递给VM的参数错误。
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
	CAUSE_INSTRUCTION_ADDRESS_MISALIGNED = 0  // 异常：指令地址未对齐。
	CAUSE_INSTRUCTION_ACCESS_FAULT       = 1  // 异常：指令访问故障（例如，从无执行权限的内存区域取指）。
	CAUSE_ILLEGAL_INSTRUCTION            = 2  // 异常：遇到非法或不支持的指令。
	CAUSE_BREAKPOINT                     = 3  // 异常：执行 `EBREAK` 指令。
	CAUSE_LOAD_ADDRESS_MISALIGNED        = 4  // 异常：加载地址未对齐。
	CAUSE_LOAD_ACCESS_FAULT              = 5  // 异常：加载访问故障（例如，从无效地址加载）。
	CAUSE_STORE_ADDRESS_MISALIGNED       = 6  // 异常：存储或AMO（原子操作）地址未对齐。
	CAUSE_STORE_ACCESS_FAULT             = 7  // 异常：存储或AMO访问故障。
	CAUSE_USER_ECALL                     = 8  // 异常：在用户模式下执行 `ECALL`。
	CAUSE_SUPERVISOR_ECALL               = 9  // 异常：在监控模式下执行 `ECALL`。
	CAUSE_MACHINE_ECALL                  = 11 // 异常：在机器模式下执行 `ECALL`。
	CAUSE_INSTRUCTION_PAGE_FAULT         = 12 // 页错误：取指时发生。
	CAUSE_LOAD_PAGE_FAULT                = 13 // 页错误：加载时发生。
	CAUSE_STORE_PAGE_FAULT               = 15 // 页错误：存储时发生。
)

// --- CSR (Control and Status Register) 地址 ---
// 定义了标准RISC-V特权级规范中的一些常用CSR的地址。
const (
	CSR_FFLAGS = 0x001 // F: 浮点异常标志。
	CSR_FRM    = 0x002 // F: 浮点动态舍入模式。
	CSR_FCSR   = 0x003 // F: 浮点控制和状态寄存器（FRM + FFLAGS）。
	CSR_VSTART = 0x008 // V: 向量操作的起始元素索引。

	// --- 监控模式 CSRs ---
	CSR_SSTATUS    = 0x100 // S: 监控模式状态寄存器。
	CSR_SIE        = 0x104 // S: 监控模式中断使能寄存器。
	CSR_STVEC      = 0x105 // S: 监控模式陷阱处理程序基地址。
	CSR_SCOUNTEREN = 0x106 // S: 监控模式计数器使能寄存器。
	CSR_SSCRATCH   = 0x140 // S: 供监控模式陷阱处理程序使用的暂存寄存器。
	CSR_SEPC       = 0x141 // S: 监控模式异常程序计数器。
	CSR_SCAUSE     = 0x142 // S: 监控模式陷阱原因。
	CSR_STVAL      = 0x143 // S: 监控模式坏地址或指令。
	CSR_SIP        = 0x144 // S: 监控模式中断挂起。
	CSR_SATP       = 0x180 // S: 监控模式地址翻译与保护。

	// --- 机器模式 CSRs ---
	CSR_MSTATUS   = 0x300 // M: 机器状态寄存器。
	CSR_MISA      = 0x301 // M: 机器ISA（指令集架构）和扩展。
	CSR_MEDELEG   = 0x302 // M: 机器异常委托寄存器。
	CSR_MIDELEG   = 0x303 // M: 机器中断委托寄存器。
	CSR_MIE       = 0x304 // M: 机器中断使能。
	CSR_MTVEC     = 0x305 // M: 机器陷阱处理程序基地址。
	CSR_MSCRATCH  = 0x340 // M: 供机器陷阱处理程序使用的暂存寄存器。
	CSR_MEPC      = 0x341 // M: 机器异常程序计数器。
	CSR_MCAUSE    = 0x342 // M: 机器陷阱原因。
	CSR_MTVAL     = 0x343 // M: 机器陷阱值（例如，出错的地址或指令）。
	CSR_MIP       = 0x344 // M: 机器中断挂起。
	CSR_MHARTID   = 0xF14 // M: 机器模式硬件线程ID（只读）。
	CSR_MVENDORID = 0xF11 // M: 供应商ID（只读）。

	// --- 机器模式性能计数器 CSRs ---
	CSR_MCYCLE          = 0xb00 // M: 机器模式周期计数器（低32位）。
	CSR_MCYCLEH         = 0xb80 // M: 机器模式周期计数器（高32位）。
	CSR_MINSTRET        = 0xb02 // M: 机器模式指令执行计数器（低32位）。
	CSR_MINSTRETH       = 0xb82 // M: 机器模式指令执行计数器（高32位）。
	CSR_MCYCLEH_ALIAS   = 0x3a0 // M: mcycleh 的别名（RV32）。
	CSR_MINSTRETH_ALIAS = 0x3a2 // M: minstreth 的别名（RV32）。
	CSR_MCOUNTINHIBIT   = 0x3b0 // M: 机器模式计数器禁止寄存器。

	// --- 向量扩展 CSRs ---
	CSR_VL    = 0xC20 // V: 当前向量指令处理的元素数量。
	CSR_VTYPE = 0xC21 // V: 向量数据类型和分组配置。
	CSR_VLENB = 0xC22 // V: 向量寄存器长度（以字节为单位）。
)

// --- MSTATUS 和 SSTATUS 寄存器的位域 ---
const (
	// SSTATUS 字段
	SSTATUS_SIE = 1 << 1 // 监控模式中断使能
	SSTATUS_SPP = 1 << 8 // 监控模式先前特权级

	// MSTATUS 字段
	MSTATUS_MIE = 1 << 3  // 机器模式中断使能
	MSTATUS_MPP = 3 << 11 // 机器模式先前特权级 (2 bits)
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
	OPCODE_LUI      = 0x37 // U-Type: LUI (加载高位立即数)。
	OPCODE_AUIPC    = 0x17 // U-Type: AUIPC (将高位立即数加到PC上)。
	OPCODE_JAL      = 0x6F // J-Type: JAL (跳转并链接)。
	OPCODE_JALR     = 0x67 // I-Type: JALR (寄存器跳转并链接)。
	OPCODE_BRANCH   = 0x63 // B-Type: 条件分支指令 (BEQ, BNE 等)。
	OPCODE_LOAD     = 0x03 // I-Type: 加载指令 (LB, LH, LW 等)。
	OPCODE_STORE    = 0x23 // S-Type: 存储指令 (SB, SH, SW)。
	OPCODE_OP_IMM   = 0x13 // I-Type: 立即数算术/逻辑指令 (ADDI, SLTI 等)。
	OPCODE_OP       = 0x33 // R-Type: 寄存器-寄存器算术/逻辑指令 (ADD, SUB 等)。
	OPCODE_FENCE    = 0x0F // I-Type: FENCE, FENCE.I 等内存排序指令。
	OPCODE_SYSTEM   = 0x73 // I-Type: 系统指令 (ECALL, EBREAK, CSRs)。
	OPCODE_LOAD_FP  = 0x07 // F-扩展: 浮点加载指令 (FLW)。
	OPCODE_STORE_FP = 0x27 // F-扩展: 浮点存储指令 (FSW)。
	OPCODE_OP_FP    = 0x53 // F-扩展: 浮点计算指令。
	OPCODE_MADD     = 0x43 // F-扩展 (FMA): FMADD.S/D
	OPCODE_MSUB     = 0x47 // F-扩展 (FMA): FMSUB.S/D
	OPCODE_NMSUB    = 0x4B // F-扩展 (FMA): FNMSUB.S/D
	OPCODE_NMADD    = 0x4F // F-扩展 (FMA): FNMADD.S/D
	OPCODE_VECTOR   = 0x57 // V-扩展: 向量指令。
	OPCODE_AMO      = 0x2F // A-扩展: 原子内存操作。
)

// --- Funct3 字段常量 ---

// 用于 BRANCH 操作码
const (
	FUNCT3_BEQ  = 0 // BEQ (如果相等则分支)
	FUNCT3_BNE  = 1 // BNE (如果不相等则分支)
	FUNCT3_BLT  = 4 // BLT (如果有符号小于则分支)
	FUNCT3_BGE  = 5 // BGE (如果有符号大于或等于则分支)
	FUNCT3_BLTU = 6 // BLTU (如果无符号小于则分支)
	FUNCT3_BGEU = 7 // BGEU (如果无符号大于或等于则分支)
)

// 用于 LOAD 操作码
const (
	FUNCT3_LB  = 0 // LB (有符号加载字节)
	FUNCT3_LH  = 1 // LH (有符号加载半字)
	FUNCT3_LW  = 2 // LW (加载字)
	FUNCT3_LBU = 4 // LBU (无符号加载字节)
	FUNCT3_LHU = 5 // LHU (无符号加载半字)
)

// 用于 STORE 操作码
const (
	FUNCT3_SB = 0 // SB (存储字节)
	FUNCT3_SH = 1 // SH (存储半字)
	FUNCT3_SW = 2 // SW (存储字)
)

// 用于 OP_IMM 和 OP 操作码
const (
	FUNCT3_ADD_SUB = 0 // ADD/SUB (OP) / ADDI (OP_IMM) 加/减
	FUNCT3_SLL     = 1 // SLL (OP) / SLLI (OP_IMM) 逻辑左移
	FUNCT3_SLT     = 2 // SLT (OP) / SLTI (OP_IMM) 有符号小于则置位
	FUNCT3_SLTU    = 3 // SLTU (OP) / SLTIU (OP_IMM) 无符号小于则置位
	FUNCT3_XOR     = 4 // XOR (OP) / XORI (OP_IMM) 异或
	FUNCT3_SRL_SRA = 5 // SRL/SRA (OP) / SRLI/SRAI (OP_IMM) 逻辑/算术右移
	FUNCT3_OR      = 6 // OR (OP) / ORI (OP_IMM) 或
	FUNCT3_AND     = 7 // AND (OP) / ANDI (OP_IMM) 与
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
	FUNCT3_AMO_W = 2 // 用于AMO的*.W (32位)
)
const (
	FUNCT5_LR      = 0b00010 // LR (预留加载)
	FUNCT5_SC      = 0b00011 // SC (条件存储)
	FUNCT5_AMOSWAP = 0b00001 // AMOSWAP (原子交换)
	FUNCT5_AMOADD  = 0b00000 // AMOADD (原子加)
	FUNCT5_AMOXOR  = 0b00100 // AMOXOR (原子异或)
	FUNCT5_AMOAND  = 0b01100 // AMOAND (原子与)
	FUNCT5_AMOOR   = 0b01000 // AMOOR (原子或)
	FUNCT5_AMOMIN  = 0b10000 // AMOMIN (有符号原子最小)
	FUNCT5_AMOMAX  = 0b10100 // AMOMAX (有符号原子最大)
	FUNCT5_AMOMINU = 0b11000 // AMOMINU (无符号原子最小)
	FUNCT5_AMOMAXU = 0b11100 // AMOMAXU (无符号原子最大)
)

// 用于 F/D-扩展 (浮点) LOAD/STORE
const (
	FUNCT3_FLW = 2 // FLW (浮点加载单字)
	FUNCT3_FLD = 3 // FLD (浮点加载双字)
	FUNCT3_FSW = 2 // FSW (浮点存储单字)
	FUNCT3_FSD = 3 // FSD (浮点存储双字)
)

// 用于 F/D-扩展 OP_FP 的 Funct7
const (
	FUNCT7_FADD_S     = 0b0000000 // FADD.S (单精度浮点加法)
	FUNCT7_FSUB_S     = 0b0000100 // FSUB.S (单精度浮点减法)
	FUNCT7_FMUL_S     = 0b0001000 // FMUL.S (单精度浮点乘法)
	FUNCT7_FDIV_S     = 0b0001100 // FDIV.S (单精度浮点除法)
	FUNCT7_FSQRT_S    = 0b0101100 // FSQRT.S (单精度浮点平方根)
	FUNCT7_FSGNJ_S    = 0b0010000 // FSGNJ.S, FSGNJN.S, FSGNJX.S (单精度浮点符号注入)
	FUNCT7_FMIN_MAX_S = 0b0010100 // FMIN.S, FMAX.S (单精度浮点最小/最大值)

	FUNCT7_FCVT_W_S = 0b1100000 // FCVT.W.S, FCVT.WU.S (单精度浮点转整数)
	FUNCT7_FCVT_W_D = 0b1100001 // FCVT.W.D, FCVT.WU.D (双精度浮点转整数)
	FUNCT7_FCVT_S_W = 0b1101000 // FCVT.S.W, FCVT.S.WU (整数转单精度浮点)
	FUNCT7_FCVT_D_W = 0b1101001 // FCVT.D.W, FCVT.D.WU (整数转双精度浮点)
	FUNCT7_FCVT_S_D = 0b0100000 // FCVT.S.D (双精度转单精度)
	FUNCT7_FCVT_D_S = 0b0100001 // FCVT.D.S (单精度转双精度)

	FUNCT7_FMV_X_W_FCLASS_S = 0b1110000 // FMV.X.W, FCLASS.S (移动/分类单精度浮点)
	FUNCT7_FEQ_FLT_FLE_S    = 0b1010000 // FEQ.S, FLT.S, FLE.S (单精度浮点比较)
	FUNCT7_FMV_W_X          = 0b1111000 // FMV.W.X (移动到浮点寄存器)

	// D-扩展的 funct7 LSB 设置为1
	FUNCT7_FADD_D        = 0b0000001 // FADD.D (双精度浮点加法)
	FUNCT7_FSUB_D        = 0b0000101 // FSUB.D (双精度浮点减法)
	FUNCT7_FMUL_D        = 0b0001001 // FMUL.D (双精度浮点乘法)
	FUNCT7_FDIV_D        = 0b0001101 // FDIV.D (双精度浮点除法)
	FUNCT7_FSQRT_D       = 0b0101101 // FSQRT.D (双精度浮点平方根)
	FUNCT7_FSGNJ_D       = 0b0010001 // FSGNJ.D, FSGNJN.D, FSGNJX.D (双精度浮点符号注入)
	FUNCT7_FMIN_MAX_D    = 0b0010101 // FMIN.D, FMAX.D (双精度浮点最小/最大值)
	FUNCT7_FMV_X_D       = 0b1110001 // FMV.X.D (移动双精度浮点)
	FUNCT7_FEQ_FLT_FLE_D = 0b1010001 // FEQ.D, FLT.D, FLE.D (双精度浮点比较)
	FUNCT7_FCLASS_D      = 0b1110001 // FCLASS.D (分类双精度浮点)
	FUNCT7_FMV_D_X       = 0b1111001 // FMV.D.X (移动到浮点寄存器)
)

// 用于 F-扩展 的 Funct3 (当 Funct7 确定大类后)
const (
	FUNCT3_FSGNJ_S  = 0 // FSGNJ.S (拷贝符号)
	FUNCT3_FSGNJN_S = 1 // FSGNJN.S (反转符号)
	FUNCT3_FSGNJX_S = 2 // FSGNJX.S (异或符号)

	FUNCT3_FMIN_S = 0 // FMIN.S (最小值)
	FUNCT3_FMAX_S = 1 // FMAX.S (最大值)

	FUNCT3_FLE_S = 0 // FLE.S (小于或等于)
	FUNCT3_FLT_S = 1 // FLT.S (小于)
	FUNCT3_FEQ_S = 2 // FEQ.S (等于)

	FUNCT3_FMV_X_W  = 0 // FMV.X.W (移动到整数寄存器)
	FUNCT3_FCLASS_S = 1 // FCLASS.S (分类)
)

// 用于 D-扩展 的 Funct3 (当 Funct7 确定大类后)
// 注意：许多 D-扩展 的 funct3 与 F-扩展 共享相同的值。
// 为清晰起见，在此处重新定义它们。
const (
	FUNCT3_FSGNJ_D  = 0 // FSGNJ.D (拷贝符号)
	FUNCT3_FSGNJN_D = 1 // FSGNJN.D (反转符号)
	FUNCT3_FSGNJX_D = 2 // FSGNJX.D (异或符号)

	FUNCT3_FMIN_D = 0 // FMIN.D (最小值)
	FUNCT3_FMAX_D = 1 // FMAX.D (最大值)

	FUNCT3_FLE_D = 0 // FLE.D (小于或等于)
	FUNCT3_FLT_D = 1 // FLT.D (小于)
	FUNCT3_FEQ_D = 2 // FEQ.D (等于)

	FUNCT3_FMV_X_D  = 0 // FMV.X.D (移动到整数寄存器)
	FUNCT3_FCLASS_D = 1 // FCLASS.D (分类)
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
	FUNCT3_OPIVV = 0b000 // 向量-向量 (整数)
	FUNCT3_OPFVV = 0b001 // 向量-向量 (浮点)
	FUNCT3_OPIVI = 0b011 // 向量-立即数 (整数)
	FUNCT3_OPFVF = 0b101 // 向量-标量 (浮点)
	FUNCT3_OPIVX = 0b110 // 向量-标量 (整数)
	FUNCT3_OP_V  = 7     // vsetvl/vsetvli (向量配置指令)
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
	FUNCT3_SYSTEM_ECALL_EBREAK = 0 // ECALL/EBREAK (系统调用/断点)
	FUNCT3_CSRRW               = 1 // CSRRW (原子读写CSR)
	FUNCT3_CSRRS               = 2 // CSRRS (原子读并置位CSR)
	FUNCT3_CSRRC               = 3 // CSRRC (原子读并清除CSR)
	FUNCT3_CSRRWI              = 5 // CSRRWI (立即数原子读写CSR)
	FUNCT3_CSRRSI              = 6 // CSRRSI (立即数原子读并置位CSR)
	FUNCT3_CSRRCI              = 7 // CSRRCI (立即数原子读并清除CSR)
)

// --- 特权指令常量 ---
// 这些常量定义了在 SYSTEM 操作码下，通过 funct12 区分的特权指令。
const (
	FUNCT12_SRET       = 0x102 // SRET 指令 (从监控模式返回)
	FUNCT12_MRET       = 0x302 // MRET 指令 (从机器模式返回)
	FUNCT12_WFI        = 0x105 // WFI 指令 (等待中断)
	FUNCT12_SFENCE_VMA = 0x120 // SFENCE.VMA 指令 (虚拟内存栅栏)
)

// --- RISC-V 压缩指令 (C-扩展) Opcode & Funct3 ---
// 这些是 RV32C 16位压缩指令集的定义。

// 压缩指令主操作码 (Quadrant)
const (
	OPCODE_C0 = 0 // Quadrant 0 (象限0)
	OPCODE_C1 = 1 // Quadrant 1 (象限1)
	OPCODE_C2 = 2 // Quadrant 2 (象限2)
)

// 用于 OPCODE_C0 的 Funct3
const (
	FUNCT3_C_ADDI4SPN = 0 // C.ADDI4SPN: 将非零立即数乘以4加到sp
	FUNCT3_C_FLD      = 1 // C.FLD: 浮点加载双字 (RV64/128)
	FUNCT3_C_LW       = 2 // C.LW: 加载字
	FUNCT3_C_FLW      = 3 // C.FLW: 浮点加载单字 (RV32)
	FUNCT3_C_FSD      = 5 // C.FSD: 浮点存储双字 (RV64/128)
	FUNCT3_C_SW       = 6 // C.SW: 存储字
	FUNCT3_C_FSW      = 7 // C.FSW: 浮点存储单字 (RV32)
)

// 用于 OPCODE_C1 的 Funct3
const (
	FUNCT3_C_NOP_ADDI     = 0 // C.NOP (空操作) / C.ADDI (立即数加法)
	FUNCT3_C_JAL          = 1 // C.JAL: 跳转并链接 (RV32)
	FUNCT3_C_LI           = 2 // C.LI: 加载立即数
	FUNCT3_C_LUI_ADDI16SP = 3 // C.LUI (加载高位立即数) / C.ADDI16SP (将立即数乘以16加到sp)
	FUNCT3_C_MISC_ALU     = 4 // 各种算术逻辑指令
	FUNCT3_C_J            = 5 // C.J: 跳转
	FUNCT3_C_BEQZ         = 6 // C.BEQZ: 如果等于零则分支
	FUNCT3_C_BNEZ         = 7 // C.BNEZ: 如果不等于零则分支
)

// 用于 OPCODE_C2 的 Funct3
const (
	FUNCT3_C_SLLI      = 0 // C.SLLI: 逻辑左移立即数
	FUNCT3_C_FLDSP     = 1 // C.FLDSP: 浮点加载双字 (sp相对) (RV64/128)
	FUNCT3_C_LWSP      = 2 // C.LWSP: 加载字 (sp相对)
	FUNCT3_C_FLWSP     = 3 // C.FLWSP: 浮点加载单字 (sp相对) (RV32)
	FUNCT3_C_JR_MV_ADD = 4 // C.JR/C.MV/C.EBREAK/C.JALR/C.ADD
	FUNCT3_C_FSDSP     = 5 // C.FSDSP: 浮点存储双字 (sp相对) (RV64/128)
	FUNCT3_C_SWSP      = 6 // C.SWSP: 存储字 (sp相对)
	FUNCT3_C_FSWSP     = 7 // C.FSWSP: 浮点存储单字 (sp相对) (RV32)
)

// C1象限 funct3=4 (FUNCT3_C_MISC_ALU) 内部的 Funct2
const (
	FUNCT2_C_SRLI    = 0 // C.SRLI: 逻辑右移立即数
	FUNCT2_C_SRAI    = 1 // C.SRAI: 算术右移立即数
	FUNCT2_C_ANDI    = 2 // C.ANDI: 与立即数
	FUNCT2_C_REG_ALU = 3 // C.SUB/C.XOR/C.OR/C.AND 等寄存器-寄存器操作
)
