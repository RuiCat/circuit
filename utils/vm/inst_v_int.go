package vm

import "encoding/binary"

// init 初始化整数向量指令的映射表。
// 这种模式将指令的 funct6 码与其实现解耦，使得代码更清晰、易于扩展。
func init() {
	// OPIVI (vector-immediate) 指令处理器
	OpiviHandlers[FUNCT6_VADD] = vadd_vi
	OpiviHandlers[FUNCT6_VRSUB] = vrsub_vi
	OpiviHandlers[FUNCT6_VAND] = vand_vi
	OpiviHandlers[FUNCT6_VOR] = vor_vi
	OpiviHandlers[FUNCT6_VXOR] = vxor_vi
	OpiviHandlers[FUNCT6_VSLL] = vshift_vi
	OpiviHandlers[FUNCT6_VSRL] = vshift_vi
	OpiviHandlers[FUNCT6_VSRA] = vshift_vi

	// OPIVX (vector-scalar) 指令处理器
	OpivxHandlers[FUNCT6_VADD] = vadd_vx
	OpivxHandlers[FUNCT6_VSUB] = vsub_vx
	OpivxHandlers[FUNCT6_VAND] = vand_vx
	OpivxHandlers[FUNCT6_VOR] = vor_vx
	OpivxHandlers[FUNCT6_VXOR] = vxor_vx
	OpivxHandlers[FUNCT6_VSLL] = vshift_vx
	OpivxHandlers[FUNCT6_VSRL] = vshift_vx
	OpivxHandlers[FUNCT6_VSRA] = vshift_vx

}

// --- OPIVI (向量-立即数) 处理器实现 ---
// 每个函数处理一种特定的算术或逻辑运算，并支持不同的元素宽度 (SEW)。

// vadd_vi 处理向量-立即数加法。
func vadd_vi(vmst *VmState, vd, vs2, i, sew_bytes, imm, _ uint32) {
	addr2 := vmst.GetVelementAddr(vs2, i, sew_bytes)
	addr_dest := vmst.GetVelementAddr(vd, i, sew_bytes)
	op1 := imm
	var op2, result uint32
	switch sew_bytes {
	case 1:
		op2 = uint32(vmst.Core.Vregs[addr2])
		result = op1 + op2
		vmst.Core.Vregs[addr_dest] = byte(result)
	case 2:
		op2 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
		result = op1 + op2
		binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
	case 4:
		op2 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
		result = op1 + op2
		binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
	}
}

// vrsub_vi 处理向量-立即数逆向减法 (imm - vector)。
func vrsub_vi(vmst *VmState, vd, vs2, i, sew_bytes, imm, _ uint32) {
	addr2 := vmst.GetVelementAddr(vs2, i, sew_bytes)
	addr_dest := vmst.GetVelementAddr(vd, i, sew_bytes)
	op1 := imm
	var op2, result uint32
	switch sew_bytes {
	case 1:
		op2 = uint32(vmst.Core.Vregs[addr2])
		result = op1 - op2
		vmst.Core.Vregs[addr_dest] = byte(result)
	case 2:
		op2 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
		result = op1 - op2
		binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
	case 4:
		op2 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
		result = op1 - op2
		binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
	}
}

// vand_vi 处理向量-立即数按位与。
func vand_vi(vmst *VmState, vd, vs2, i, sew_bytes, imm, _ uint32) {
	addr2 := vmst.GetVelementAddr(vs2, i, sew_bytes)
	addr_dest := vmst.GetVelementAddr(vd, i, sew_bytes)
	op1 := imm
	var op2, result uint32
	switch sew_bytes {
	case 1:
		op2 = uint32(vmst.Core.Vregs[addr2])
		result = op1 & op2
		vmst.Core.Vregs[addr_dest] = byte(result)
	case 2:
		op2 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
		result = op1 & op2
		binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
	case 4:
		op2 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
		result = op1 & op2
		binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
	}
}

// vor_vi 处理向量-立即数按位或。
func vor_vi(vmst *VmState, vd, vs2, i, sew_bytes, imm, _ uint32) {
	addr2 := vmst.GetVelementAddr(vs2, i, sew_bytes)
	addr_dest := vmst.GetVelementAddr(vd, i, sew_bytes)
	op1 := imm
	var op2, result uint32
	switch sew_bytes {
	case 1:
		op2 = uint32(vmst.Core.Vregs[addr2])
		result = op1 | op2
		vmst.Core.Vregs[addr_dest] = byte(result)
	case 2:
		op2 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
		result = op1 | op2
		binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
	case 4:
		op2 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
		result = op1 | op2
		binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
	}
}

// vxor_vi 处理向量-立即数按位异或。
func vxor_vi(vmst *VmState, vd, vs2, i, sew_bytes, imm, _ uint32) {
	addr2 := vmst.GetVelementAddr(vs2, i, sew_bytes)
	addr_dest := vmst.GetVelementAddr(vd, i, sew_bytes)
	op1 := imm
	var op2, result uint32
	switch sew_bytes {
	case 1:
		op2 = uint32(vmst.Core.Vregs[addr2])
		result = op1 ^ op2
		vmst.Core.Vregs[addr_dest] = byte(result)
	case 2:
		op2 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
		result = op1 ^ op2
		binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
	case 4:
		op2 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
		result = op1 ^ op2
		binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
	}
}

// vshift_vi 处理所有向量-立即数移位操作（左移、逻辑右移、算术右移）。
func vshift_vi(vmst *VmState, vd, vs2, i, sew_bytes, _, imm5 uint32) {
	ir := vmst.lastIR // 从 vm state 获取当前指令
	funct6 := ir >> 26
	shamt := imm5 // 移位量
	addr2 := vmst.GetVelementAddr(vs2, i, sew_bytes)
	addr_dest := vmst.GetVelementAddr(vd, i, sew_bytes)
	var op2, result uint32
	switch sew_bytes {
	case 1: // 8-bit
		op2 = uint32(vmst.Core.Vregs[addr2])
		shamt &= 0x7 // 取模 8
		switch funct6 {
		case FUNCT6_VSLL: // 逻辑左移
			result = op2 << shamt
		case FUNCT6_VSRL: // 逻辑右移
			result = uint32(byte(op2) >> shamt)
		default: // VSRA - 算术右移
			result = uint32(int8(op2) >> shamt)
		}
		vmst.Core.Vregs[addr_dest] = byte(result)
	case 2: // 16-bit
		op2 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
		shamt &= 0xF // 取模 16
		switch funct6 {
		case FUNCT6_VSLL:
			result = op2 << shamt
		case FUNCT6_VSRL:
			result = uint32(uint16(op2) >> shamt)
		default: // VSRA
			result = uint32(int16(op2) >> shamt)
		}
		binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
	case 4: // 32-bit
		op2 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
		shamt &= 0x1F // 取模 32
		switch funct6 {
		case FUNCT6_VSLL:
			result = op2 << shamt
		case FUNCT6_VSRL:
			result = op2 >> shamt
		default: // VSRA
			result = uint32(int32(op2) >> shamt)
		}
		binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
	}
}

// --- OPIVX (向量-标量) 处理器实现 ---
// op2 是从标量整数寄存器 rs1 读取的值。

// vadd_vx 处理向量-标量加法。
func vadd_vx(vmst *VmState, vd, vs2, i, sew_bytes, op2 uint32) {
	addr1 := vmst.GetVelementAddr(vs2, i, sew_bytes)
	addr_dest := vmst.GetVelementAddr(vd, i, sew_bytes)
	var op1, result uint32
	switch sew_bytes {
	case 1:
		op1 = uint32(vmst.Core.Vregs[addr1])
		result = op1 + op2
		vmst.Core.Vregs[addr_dest] = byte(result)
	case 2:
		op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr1:]))
		result = op1 + op2
		binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
	case 4:
		op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr1:])
		result = op1 + op2
		binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
	}
}

// vsub_vx 处理向量-标量减法 (vector - scalar)。
func vsub_vx(vmst *VmState, vd, vs2, i, sew_bytes, op2 uint32) {
	addr1 := vmst.GetVelementAddr(vs2, i, sew_bytes)
	addr_dest := vmst.GetVelementAddr(vd, i, sew_bytes)
	var op1, result uint32
	switch sew_bytes {
	case 1:
		op1 = uint32(vmst.Core.Vregs[addr1])
		result = op1 - op2
		vmst.Core.Vregs[addr_dest] = byte(result)
	case 2:
		op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr1:]))
		result = op1 - op2
		binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
	case 4:
		op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr1:])
		result = op1 - op2
		binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
	}
}

// vand_vx 处理向量-标量按位与。
func vand_vx(vmst *VmState, vd, vs2, i, sew_bytes, op2 uint32) {
	addr1 := vmst.GetVelementAddr(vs2, i, sew_bytes)
	addr_dest := vmst.GetVelementAddr(vd, i, sew_bytes)
	var op1, result uint32
	switch sew_bytes {
	case 1:
		op1 = uint32(vmst.Core.Vregs[addr1])
		result = op1 & op2
		vmst.Core.Vregs[addr_dest] = byte(result)
	case 2:
		op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr1:]))
		result = op1 & op2
		binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
	case 4:
		op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr1:])
		result = op1 & op2
		binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
	}
}

// vor_vx 处理向量-标量按位或。
func vor_vx(vmst *VmState, vd, vs2, i, sew_bytes, op2 uint32) {
	addr1 := vmst.GetVelementAddr(vs2, i, sew_bytes)
	addr_dest := vmst.GetVelementAddr(vd, i, sew_bytes)
	var op1, result uint32
	switch sew_bytes {
	case 1:
		op1 = uint32(vmst.Core.Vregs[addr1])
		result = op1 | op2
		vmst.Core.Vregs[addr_dest] = byte(result)
	case 2:
		op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr1:]))
		result = op1 | op2
		binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
	case 4:
		op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr1:])
		result = op1 | op2
		binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
	}
}

// vxor_vx 处理向量-标量按位异或。
func vxor_vx(vmst *VmState, vd, vs2, i, sew_bytes, op2 uint32) {
	addr1 := vmst.GetVelementAddr(vs2, i, sew_bytes)
	addr_dest := vmst.GetVelementAddr(vd, i, sew_bytes)
	var op1, result uint32
	switch sew_bytes {
	case 1:
		op1 = uint32(vmst.Core.Vregs[addr1])
		result = op1 ^ op2
		vmst.Core.Vregs[addr_dest] = byte(result)
	case 2:
		op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr1:]))
		result = op1 ^ op2
		binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
	case 4:
		op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr1:])
		result = op1 ^ op2
		binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
	}
}

// vshift_vx 处理所有向量-标量移位操作。
func vshift_vx(vmst *VmState, vd, vs2, i, sew_bytes, op2 uint32) {
	ir := vmst.lastIR
	funct6 := ir >> 26
	addr1 := vmst.GetVelementAddr(vs2, i, sew_bytes)
	addr_dest := vmst.GetVelementAddr(vd, i, sew_bytes)
	var op1, result, shamt uint32
	switch sew_bytes {
	case 1: // 8-bit
		op1 = uint32(vmst.Core.Vregs[addr1])
		shamt = op2 & 0x7 // 移位量来自标量寄存器 rs1
		switch funct6 {
		case FUNCT6_VSLL:
			result = op1 << shamt
		case FUNCT6_VSRL:
			result = uint32(byte(op1) >> shamt)
		default: // VSRA
			result = uint32(int8(op1) >> shamt)
		}
		vmst.Core.Vregs[addr_dest] = byte(result)
	case 2: // 16-bit
		op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr1:]))
		shamt = op2 & 0xf
		switch funct6 {
		case FUNCT6_VSLL:
			result = op1 << shamt
		case FUNCT6_VSRL:
			result = uint32(uint16(op1) >> shamt)
		default: // VSRA
			result = uint32(int16(op1) >> shamt)
		}
		binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
	case 4: // 32-bit
		op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr1:])
		shamt = op2 & 0x1f
		switch funct6 {
		case FUNCT6_VSLL:
			result = op1 << shamt
		case FUNCT6_VSRL:
			result = op1 >> shamt
		default: // VSRA
			result = uint32(int32(op1) >> shamt)
		}
		binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
	}
}

// --- 主向量处理器 ---

// handleOPIVI 是 OPIVI (向量-立即数) 整数指令的主分发函数。
// 它解码指令，验证参数，并为每个向量元素调用适当的处理器。
func (vmst *VmState) handleOPIVI(ir uint32) int32 {
	// --- 解码 ---
	funct6 := ir >> 26
	vm := (ir >> 25) & 1
	vd := (ir >> 7) & 0x1f
	imm5 := (ir >> 15) & 0x1f // 5-bit 立即数
	vs2 := (ir >> 20) & 0x1f
	imm := uint32(int32(imm5<<27) >> 27) // 符号扩展
	sew_val := (vmst.Core.Vtype >> 2) & 0x7
	sew_bytes := uint32(1 << sew_val)

	// --- 验证和查找处理器 ---
	if sew_bytes > 4 {
		return CAUSE_ILLEGAL_INSTRUCTION
	}
	handler, ok := OpiviHandlers[funct6]
	if !ok {
		return CAUSE_ILLEGAL_INSTRUCTION
	}

	// --- 循环执行 ---
	vmst.lastIR = ir // 存储当前指令，供移位操作使用
	for i := vmst.Core.Vstart; i < vmst.Core.Vl; i++ {
		// 掩码处理
		if vm == 0 {
			mask_byte_index := i / 8
			mask_bit_index := i % 8
			if (vmst.Core.Vregs[mask_byte_index] & (1 << mask_bit_index)) == 0 {
				continue
			}
		}
		// 调用具体的操作函数
		handler(vmst, vd, vs2, i, sew_bytes, imm, imm5)
	}

	vmst.Core.Vstart = 0
	return 0
}

// handleOPIVX 是 OPIVX (向量-标量) 整数指令的主分发函数。
func (vmst *VmState) handleOPIVX(ir uint32) int32 {
	// --- 解码 ---
	funct6 := ir >> 26
	vm := (ir >> 25) & 1
	vd := (ir >> 7) & 0x1f
	rs1id := (ir >> 15) & 0x1f // 标量寄存器
	vs2 := (ir >> 20) & 0x1f

	// --- 验证和查找处理器 ---
	sew_val := (vmst.Core.Vtype >> 2) & 0x7
	sew_bytes := uint32(1 << sew_val)
	if sew_bytes > 4 {
		return CAUSE_ILLEGAL_INSTRUCTION
	}

	op2 := vmst.Core.Regs[rs1id] // 从标量寄存器读取操作数
	handler, ok := OpivxHandlers[funct6]
	if !ok {
		return CAUSE_ILLEGAL_INSTRUCTION
	}

	// --- 循环执行 ---
	vmst.lastIR = ir
	for i := vmst.Core.Vstart; i < vmst.Core.Vl; i++ {
		// 掩码处理
		if vm == 0 {
			mask_byte_index := i / 8
			mask_bit_index := i % 8
			if (vmst.Core.Vregs[mask_byte_index] & (1 << mask_bit_index)) == 0 {
				continue
			}
		}
		handler(vmst, vd, vs2, i, sew_bytes, op2)
	}

	vmst.Core.Vstart = 0
	return 0
}
