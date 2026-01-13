package vm

import (
	"encoding/binary"
	"math"
)

// init 初始化浮点向量指令的映射表。
// 通过将 funct6 代码映射到具体的 lambda 函数，可以使指令处理更加模块化和高效。
func init() {

	// --- OPFVV (向量-向量) 浮点操作 ---
	OpfvvHandlers[FUNCT6_VFADD] = func(f1, f2 float32) float32 { return f1 + f2 }
	OpfvvHandlers[FUNCT6_VFSUB] = func(f1, f2 float32) float32 { return f1 - f2 }
	OpfvvHandlers[FUNCT6_VFMUL] = func(f1, f2 float32) float32 { return f1 * f2 }
	OpfvvHandlers[FUNCT6_VFDIV] = func(f1, f2 float32) float32 { return f1 / f2 }

	// --- OPFVF (向量-标量) 浮点操作 ---
	// 注意操作数的顺序，f1 是标量，f2 是向量元素。
	OpfvfHandlers[FUNCT6_VFADD] = func(f1, f2 float32) float32 { return f2 + f1 }
	OpfvfHandlers[FUNCT6_VFSUB] = func(f1, f2 float32) float32 { return f2 - f1 }
	OpfvfHandlers[FUNCT6_VFRSUB] = func(f1, f2 float32) float32 { return f1 - f2 }
	OpfvfHandlers[FUNCT6_VFMUL] = func(f1, f2 float32) float32 { return f2 * f1 }
	OpfvfHandlers[FUNCT6_VFDIV] = func(f1, f2 float32) float32 { return f2 / f1 }
	OpfvfHandlers[FUNCT6_VFRDIV] = func(f1, f2 float32) float32 { return f1 / f2 }

}

// handleVFPOPIVV 是 OPFVV (向量-向量) 浮点指令的主分发函数。
// 它解码指令，验证参数，并对向量中的每个元素执行操作。
func (vmst *VmState) handleVFPOPIVV(ir uint32) int32 {
	// --- 解码 ---
	funct6 := ir >> 26
	vm := (ir >> 25) & 1
	vd := (ir >> 7) & 0x1f
	vs1 := (ir >> 15) & 0x1f
	vs2 := (ir >> 20) & 0x1f

	// --- 参数验证 ---
	// 获取当前的向量元素位宽 (SEW)
	sew_val := (vmst.Core.Vtype >> 2) & 0x7
	sew_bytes := uint32(1 << sew_val)
	// 这个实现目前只支持32位浮点数 (SEW=32)
	if sew_bytes != 4 {
		return CAUSE_ILLEGAL_INSTRUCTION
	}

	// 从映射表中查找对应的处理函数
	handler, ok := OpfvvHandlers[funct6]
	if !ok {
		return CAUSE_ILLEGAL_INSTRUCTION
	}

	// --- 循环执行 ---
	// 遍历 vl 个元素，从 vstart 开始
	for i := vmst.Core.Vstart; i < vmst.Core.Vl; i++ {
		// 检查掩码 (vm=0 表示被掩码)
		if vm == 0 {
			mask_byte_index := i / 8
			mask_bit_index := i % 8
			// 如果掩码位为0，则跳过当前元素
			if (vmst.Core.Vregs[mask_byte_index] & (1 << mask_bit_index)) == 0 {
				continue
			}
		}

		// 计算源和目标元素的地址
		addr1 := vmst.GetVelementAddr(vs1, i, sew_bytes)
		addr2 := vmst.GetVelementAddr(vs2, i, sew_bytes)
		addr_dest := vmst.GetVelementAddr(vd, i, sew_bytes)

		// 读取操作数，执行操作，写回结果
		op1_bits := binary.LittleEndian.Uint32(vmst.Core.Vregs[addr1:])
		op2_bits := binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
		f1 := math.Float32frombits(op1_bits)
		f2 := math.Float32frombits(op2_bits)

		result := handler(f1, f2)
		binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], math.Float32bits(result))
	}

	vmst.Core.Vstart = 0 // 重置 vstart
	return 0
}

// handleVFPOPIVF 是 OPFVF (向量-标量) 浮点指令的主分发函数。
func (vmst *VmState) handleVFPOPIVF(ir uint32) int32 {
	// --- 解码 ---
	funct6 := ir >> 26
	vm := (ir >> 25) & 1
	vd := (ir >> 7) & 0x1f
	frs1 := (ir >> 15) & 0x1f // 注意这里是浮点寄存器 frs1
	vs2 := (ir >> 20) & 0x1f

	// --- 参数验证 ---
	sew_val := (vmst.Core.Vtype >> 2) & 0x7
	sew_bytes := uint32(1 << sew_val)
	if sew_bytes != 4 {
		return CAUSE_ILLEGAL_INSTRUCTION
	}

	// 获取标量操作数
	f1 := getFRegS(vmst, frs1)

	// 查找处理函数
	handler, ok := OpfvfHandlers[funct6]
	if !ok {
		return CAUSE_ILLEGAL_INSTRUCTION
	}

	// --- 循环执行 ---
	for i := vmst.Core.Vstart; i < vmst.Core.Vl; i++ {
		// 检查掩码
		if vm == 0 {
			mask_byte_index := i / 8
			mask_bit_index := i % 8
			if (vmst.Core.Vregs[mask_byte_index] & (1 << mask_bit_index)) == 0 {
				continue
			}
		}
		addr2 := vmst.GetVelementAddr(vs2, i, sew_bytes)
		addr_dest := vmst.GetVelementAddr(vd, i, sew_bytes)
		op2_bits := binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
		f2 := math.Float32frombits(op2_bits)

		result := handler(f1, f2)
		binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], math.Float32bits(result))
	}

	vmst.Core.Vstart = 0
	return 0
}
