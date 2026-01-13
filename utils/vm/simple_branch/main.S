.section .text
.globl _start

_start:
    lui a3, %hi(0x80001000)
    addi a3, a3, %lo(0x80001000)

    # --- BEQ a0, a1, target ---
    # Test 1: Branch taken (5 == 5)
    li a0, 5
    li a1, 5
    beq a0, a1, beq_taken
    # Not taken path
    li t0, 0 # Should not be stored
    sw t0, 0(a3)
    j beq_done
beq_taken:
    li t0, 1 # Expected value
    sw t0, 0(a3)
beq_done:

    # Test 2: Branch not taken (5 != 10)
    li a0, 5
    li a1, 10
    beq a0, a1, bne_taken_fail
    # Not taken path
    li t0, 2 # Expected value
    sw t0, 4(a3)
    j bne_done
bne_taken_fail:
    li t0, 0 # Should not be stored
    sw t0, 4(a3)
bne_done:


    # --- BNE a0, a1, target ---
    # Test 3: Branch taken (5 != 10)
    li a0, 5
    li a1, 10
    bne a0, a1, bne_taken
    # Not taken path
    li t0, 0
    sw t0, 8(a3)
    j bne_test2
bne_taken:
    li t0, 3
    sw t0, 8(a3)
bne_test2:

    # Test 4: Branch not taken (5 == 5)
    li a0, 5
    li a1, 5
    bne a0, a1, blt_taken_fail
    # Not taken path
    li t0, 4
    sw t0, 12(a3)
    j blt_done
blt_taken_fail:
    li t0, 0
    sw t0, 12(a3)
blt_done:

    # --- BLT a0, a1, target ---
    # Test 5: Branch taken (-10 < 5)
    li a0, -10
    li a1, 5
    blt a0, a1, blt_taken
    li t0, 0
    sw t0, 16(a3)
    j blt_test2
blt_taken:
    li t0, 5
    sw t0, 16(a3)
blt_test2:

    # Test 6: Branch not taken (10 < 5)
    li a0, 10
    li a1, 5
    blt a0, a1, bge_taken_fail
    li t0, 6
    sw t0, 20(a3)
    j bge_done
bge_taken_fail:
    li t0, 0
    sw t0, 20(a3)
bge_done:

    # --- BGE a0, a1, target ---
    # Test 7: Branch taken (10 >= 5)
    li a0, 10
    li a1, 5
    bge a0, a1, bge_taken
    li t0, 0
    sw t0, 24(a3)
    j bge_test2
bge_taken:
    li t0, 7
    sw t0, 24(a3)
bge_test2:

    # Test 8: Branch not taken (-10 >= 5)
    li a0, -10
    li a1, 5
    bge a0, a1, bltu_taken_fail
    li t0, 8
    sw t0, 28(a3)
    j bltu_done
bltu_taken_fail:
    li t0, 0
    sw t0, 28(a3)
bltu_done:

    # --- BLTU a0, a1, target ---
    # Test 9: Branch taken (10 < 20)
    li a0, 10
    li a1, 20
    bltu a0, a1, bltu_taken
    li t0, 0
    sw t0, 32(a3)
    j bltu_test2
bltu_taken:
    li t0, 9
    sw t0, 32(a3)
bltu_test2:

    # Test 10: Branch not taken (-1 (unsigned) < 20)
    li a0, -1
    li a1, 20
    bltu a0, a1, bgeu_taken_fail
    li t0, 10
    sw t0, 36(a3)
    j bgeu_done
bgeu_taken_fail:
    li t0, 0
    sw t0, 36(a3)
bgeu_done:

    # --- BGEU a0, a1, target ---
    # Test 11: Branch taken (-1 (unsigned) >= 20)
    li a0, -1
    li a1, 20
    bgeu a0, a1, bgeu_taken
    li t0, 0
    sw t0, 40(a3)
    j bgeu_test2
bgeu_taken:
    li t0, 11
    sw t0, 40(a3)
bgeu_test2:

    # Test 12: Branch not taken (10 >= 20)
    li a0, 10
    li a1, 20
    bgeu a0, a1, hang
    li t0, 12
    sw t0, 44(a3)

hang:
    j hang
