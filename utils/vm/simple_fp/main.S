.section .data
.align 2
# --- 初始数据 ---
val1_f32: .float 3.14      # pi
val2_f32: .float 1.57      # pi / 2
val3_int: .word  42        # 整数值
val4_f32: .float -3.14
val5_f32: .float 0.0
val6_f32: .float -0.0

.section .bss
.align 2
# --- 结果存储区 (80字节) ---
# 用于存储20个32位结果
result_area: .space 80

.section .text
.globl _start

_start:
    # --- 设置基地址 ---
    lui a0, %hi(val1_f32)
    addi a0, a0, %lo(val1_f32)
    lui a1, %hi(result_area)
    addi a1, a1, %lo(result_area)

    # --- 加载初始值 ---
    flw ft0, 0(a0)   # ft0 = 3.14
    flw ft1, 4(a0)   # ft1 = 1.57
    lw  t0, 8(a0)    # t0 = 42
    flw ft2, 12(a0)  # ft2 = -3.14
    flw ft3, 16(a0)  # ft3 = 0.0
    flw ft4, 20(a0)  # ft4 = -0.0

    # --- 1. 算术运算 ---
    fadd.s ft5, ft0, ft1
    fsw ft5, 0(a1)
    fsub.s ft5, ft0, ft1
    fsw ft5, 4(a1)
    fmul.s ft5, ft0, ft1
    fsw ft5, 8(a1)
    fdiv.s ft5, ft0, ft1
    fsw ft5, 12(a1)
    fsqrt.s ft5, ft0
    fsw ft5, 16(a1)

    # --- 2. 转换与移动 ---
    fcvt.s.w ft5, t0
    fsw ft5, 20(a1)
    fcvt.w.s t1, ft0
    sw t1, 24(a1)
    fmv.w.x ft5, t0
    fsw ft5, 28(a1)
    fmv.x.w t2, ft0
    sw t2, 32(a1)

    # --- 3. 比较 ---
    feq.s t3, ft0, ft1
    sw t3, 36(a1)
    feq.s t4, ft0, ft0
    sw t4, 40(a1)

    # --- 4. 新增指令测试 ---

    # FMIN.S/FMAX.S
    fmin.s ft5, ft0, ft1    # fmin(3.14, 1.57) -> 1.57
    fsw ft5, 44(a1)
    fmax.s ft5, ft0, ft2    # fmax(3.14, -3.14) -> 3.14
    fsw ft5, 48(a1)
    fmin.s ft5, ft3, ft4    # fmin(0.0, -0.0) -> -0.0
    fsw ft5, 52(a1)

    # FCLASS.S
    fclass.s t3, ft0  # Class of 3.14 (positive normal)
    sw t3, 56(a1)
    fclass.s t4, ft4  # Class of -0.0 (negative zero)
    sw t4, 60(a1)
    
    # FCVT.S.D / FCVT.D.S (模拟)
    # 假设 fd0 已经通过某种方式加载了双精度值 3.14159265
    # 这里我们用 fmv.w.x 模拟一个单精度到双精度的转换
    fcvt.d.s fa0, ft0  # fa0 = double(3.14)
    fcvt.s.d ft6, fa0  # ft6 = float(double(3.14))
    fsw ft6, 64(a1)

hang:
    j hang
