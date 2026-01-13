package vm

import (
	"encoding/binary"
	"math"
)

// getFRegD 从浮点寄存器文件中读取一个 float64。
func getFRegD(vmst *VmState, frid uint32) float64 {
	return math.Float64frombits(vmst.Core.FRegs[frid])
}

// setFRegD 将一个 float64 值写入浮点寄存器文件。
func setFRegD(vmst *VmState, frid uint32, val float64) {
	vmst.Core.FRegs[frid] = math.Float64bits(val)
}

// handleLoadFP_D 处理 FLD 指令 (双精度浮点加载)。
// 从内存中加载一个64位的值，并将其写入一个浮点寄存器。
func handleLoadFP_D(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	// --- 解码 ---
	rd := (ir >> 7) & 0x1f
	rs1 := (ir >> 15) & 0x1f
	imm := int32(ir&0xfff00000) >> 20

	// --- 计算地址 ---
	addr := vmst.Core.Regs[rs1] + uint32(imm)
	ofs_addr := addr - VmRamImageOffSet

	// --- 边界检查 ---
	if addr < VmRamImageOffSet || ofs_addr+8 > uint32(VmMemoRySize) {
		vmst.Core.Mtval = addr
		return 0, 0, 0, CAUSE_LOAD_ACCESS_FAULT
	}
	// 对齐检查 (8字节)
	if ofs_addr&7 != 0 {
		vmst.Core.Mtval = addr
		return 0, 0, 0, CAUSE_LOAD_ADDRESS_MISALIGNED
	}

	// --- 从内存加载数据 ---
	val := binary.LittleEndian.Uint64(vmst.Memory[ofs_addr:])

	// --- 写入寄存器 ---
	setFRegD(vmst, rd, math.Float64frombits(val))
	return 0, 0, pc + 4, 0
}

// handleStoreFP_D 处理 FSD 指令 (双精度浮点存储)。
// 从一个浮点寄存器中读取一个64位的值，并将其存入内存。
func handleStoreFP_D(vmst *VmState, ir uint32, pc uint32) (uint32, uint32, uint32, int32) {
	// --- 解码 ---
	rs1 := (ir >> 15) & 0x1f
	rs2 := (ir >> 20) & 0x1f
	// S-Type 立即数重组
	imm_11_5 := (ir >> 25) & 0x7f
	imm_4_0 := (ir >> 7) & 0x1f
	imm_unsigned := (imm_11_5 << 5) | imm_4_0
	imm := int32(imm_unsigned<<20) >> 20

	// --- 计算地址 ---
	addr := vmst.Core.Regs[rs1] + uint32(imm)
	ofs_addr := addr - VmRamImageOffSet

	// --- 边界检查 ---
	if addr < VmRamImageOffSet || ofs_addr+8 > uint32(VmMemoRySize) {
		vmst.Core.Mtval = addr
		return 0, 0, 0, CAUSE_STORE_ACCESS_FAULT
	}
	// 对齐检查 (8字节)
	if ofs_addr&7 != 0 {
		vmst.Core.Mtval = addr
		return 0, 0, 0, CAUSE_STORE_ADDRESS_MISALIGNED
	}

	// --- 从寄存器读取数据 ---
	val_bits := math.Float64bits(getFRegD(vmst, rs2))

	// --- 存入内存 ---
	binary.LittleEndian.PutUint64(vmst.Memory[ofs_addr:], val_bits)
	return 0, 0, pc + 4, 0
}
