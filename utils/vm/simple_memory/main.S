.section .text
.globl _start

_start:
    # 暂存区地址，用于存储和加载
    lui a3, %hi(0x80001000)
    addi a3, a3, %lo(0x80001000)

    # 结果区地址，用于存储测试结果
    lui a4, %hi(0x80001100)
    addi a4, a4, %lo(0x80001100)

    # --- 准备并存储初始数据到暂存区 ---
    li t0, 0x5678ABCD # 完整字
    li t1, 0xFFFFDEFA # 负半字
    li t2, 0xFFFFFF8A # 负字节

    sw t0, 0(a3)  # 存储字 0x5678ABCD
    sh t1, 4(a3)  # 存储半字 0xDEFA
    sb t2, 6(a3)  # 存储字节 0x8A

    # --- 从暂存区加载数据 ---
    lw  t3, 0(a3)   # t3 应为 0x5678ABCD
    lh  t4, 4(a3)   # t4 应为 0xFFFFDEFA (符号扩展)
    lhu t5, 4(a3)   # t5 应为 0x0000DEFA (零扩展)
    lb  t6, 6(a3)   # t6 应为 0xFFFFFF8A (符号扩展)
    lbu t0, 6(a3)   # t0 应为 0x0000008A (零扩展)

    # --- 将加载的值存储到结果区以供验证 ---
    sw t3, 0(a4)    # 存储 LW 结果
    sw t4, 4(a4)    # 存储 LH 结果
    sw t5, 8(a4)    # 存储 LHU 结果
    sw t6, 12(a4)   # 存储 LB 结果
    sw t0, 16(a4)   # 存储 LBU 结果

    # --- 测试存储指令到结果区 ---
    sb t6, 20(a4)   # 存储字节 0x8A
    sh t4, 22(a4)   # 存储半字 0xDEFA
    sw t3, 24(a4)   # 存储字 0x5678ABCD

hang:
    j hang
