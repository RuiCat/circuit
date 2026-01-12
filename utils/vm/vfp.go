package vm

import (
	"encoding/binary"
	"math"
)

// handleOPIVI 函数处理 RISC-V 向量扩展中的 OPIVI 编码指令，
// 这些是向量-立即数整数算术指令。
// ir: 32位的指令字。
// 返回: 成功时返回0，发生陷阱时返回非0值。
func (vmst *VmState) handleOPIVI(ir uint32) int32 {
	// 从指令中解码字段
	funct6 := ir >> 26
	vm := (ir >> 25) & 1      // 向量掩码位
	vd := (ir >> 7) & 0x1f    // 目标向量寄存器索引
	imm5 := (ir >> 15) & 0x1f // 5位立即数
	vs2 := (ir >> 20) & 0x1f  // 源向量寄存器索引

	// 对5位立即数进行符号扩展，使其成为32位有符号整数
	imm := uint32(int32(imm5<<27) >> 27)

	// 从vtype CSR中获取当前选择的元素宽度(SEW)
	sew_val := (vmst.Core.Vtype >> 2) & 0x7
	sew_bytes := uint32(1 << sew_val) // 将SEW编码转换为字节数

	// 当前实现只支持最高32位的元素宽度
	if sew_bytes > 4 {
		return CAUSE_ILLEGAL_INSTRUCTION // 触发非法指令陷阱
	}

	// 遍历向量寄存器中的每个元素，从vstart开始直到vl
	for i := vmst.Core.Vstart; i < vmst.Core.Vl; i++ {
		// 如果 vm=0，则处理掩码寄存器v0
		if vm == 0 {
			mask_byte_index := i / 8
			mask_bit_index := i % 8
			// 检查v0中对应的掩码位是否为0
			if (vmst.Core.Vregs[mask_byte_index] & (1 << mask_bit_index)) == 0 {
				continue // 如果被掩码，则跳过当前元素，不执行操作
			}
		}

		// 获取源和目标元素的地址
		addr2 := vmst.get_velement_addr(vs2, i, sew_bytes)
		addr_dest := vmst.get_velement_addr(vd, i, sew_bytes)

		var op2, result uint32
		op1 := imm // 立即数是第一个操作数

		// 根据funct6字段确定具体操作
		switch funct6 {
		case FUNCT6_VADD: // VADD.VI: 向量-立即数加法
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
		case FUNCT6_VRSUB: // VRSUB.VI: 向量-立即数逆向减法 (imm - vs2)
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
		case FUNCT6_VAND: // VAND.VI: 向量-立即数按位与
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
		case FUNCT6_VOR: // VOR.VI: 向量-立即数按位或
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
		case FUNCT6_VXOR: // VXOR.VI: 向量-立即数按位异或
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
		case FUNCT6_VSLL, FUNCT6_VSRL, FUNCT6_VSRA: // VSLL.VI, VSRL.VI, VSRA.VI: 向量-立即数移位
			shamt := imm5 // 移位量是5位无符号立即数
			var op2, result uint32
			switch sew_bytes {
			case 1:
				op2 = uint32(vmst.Core.Vregs[addr2])
				shamt &= 0x7 // 移位量对8取模
				switch funct6 {
				case FUNCT6_VSLL:
					result = op2 << shamt
				case FUNCT6_VSRL:
					result = uint32(byte(op2) >> shamt)
				default: // VSRA
					result = uint32(int8(op2) >> shamt)
				}
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op2 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				shamt &= 0xF // 移位量对16取模
				switch funct6 {
				case FUNCT6_VSLL:
					result = op2 << shamt
				case FUNCT6_VSRL:
					result = uint32(uint16(op2) >> shamt)
				default: // VSRA
					result = uint32(int16(op2) >> shamt)
				}
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op2 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				shamt &= 0x1F // 移位量对32取模
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
		default:
			return CAUSE_ILLEGAL_INSTRUCTION // 如果funct6不匹配任何已知操作，触发非法指令陷阱
		}
	}

	vmst.Core.Vstart = 0 // 指令执行完毕，将vstart重置为0
	return 0             // 成功返回
}

// handleVSETVL 处理 VSETVLI 和 VSETIVLI 指令。
// 此指令用于配置向量上下文，包括设置向量长度(vl)和向量类型(vtype)。
// ir: 32位的指令字。
// 返回: 成功时返回0，发生陷阱时返回非0值。
func (vmst *VmState) handleVSETVL(ir uint32) int32 {
	rdid := (ir >> 7) & 0x1f
	rs1id := (ir >> 15) & 0x1f
	imm12 := ir >> 20

	// 根据RISC-V向量规范，vsetvl*指令的imm[11]位必须为0。
	if (imm12 >> 11) != 0 {
		return CAUSE_ILLEGAL_INSTRUCTION // 如果不为0，触发非法指令陷阱。
	}
	vtypei := imm12 & 0x7ff // 11位的vtypei字段

	// 从vtypei字段中解码vsew, vlmul等。
	vsew := (vtypei >> 2) & 0x7
	vlmul_encoded := vtypei & 0x3
	// vta (向量尾部无关) 和 vma (向量掩码无关) 在此实现中未使用。
	// vta := (vtypei >> 5) & 1
	// vma := (vtypei >> 6) & 1
	vill_bit_from_instr := (vtypei >> 7) // 检查vtypei的保留位是否全为0

	// VLEN是向量寄存器的物理长度，单位为位。在此实现中为128位。
	const VLEN_BITS = 128

	// 验证vsew和vlmul的设置是否合法。
	sew_bits := uint32(8 << vsew)
	sew_is_valid := sew_bits == 8 || sew_bits == 16 || sew_bits == 32

	var lmul float32
	lmul_is_valid := true
	switch vlmul_encoded {
	case VLMUL_1:
		lmul = 1
	case VLMUL_2:
		lmul = 2
	case VLMUL_4:
		lmul = 4
	case VLMUL_8:
		lmul = 8
	// 小数LMUL和保留的编码在此实现中不被支持。
	default:
		lmul_is_valid = false
	}

	// 如果任何配置非法，则设置vtype的vill位，并将vl设为0，但不产生陷阱。
	if vill_bit_from_instr != 0 || !sew_is_valid || !lmul_is_valid || lmul > 8 {
		vmst.Core.Vtype = 1 << 31 // 设置vill位表示非法vtype
		vmst.Core.Vl = 0
		if rdid != 0 {
			vmst.Core.Regs[rdid] = 0 // rd也被清零
		}
		vmst.Core.Vstart = 0
		return 0 // 无陷阱
	}

	// 如果配置有效，则更新vtype CSR。
	vmst.Core.Vtype = vtypei
	vmst.Core.Vstart = 0 // vsetvl指令总是将vstart重置为0。

	// 计算VLMAX = (VLEN / SEW) * LMUL，这是给定配置下vl的最大值。
	vlmax := uint32(float32(VLEN_BITS/sew_bits) * lmul)

	// 获取请求的向量长度(AVL)。
	var avl uint32
	if rs1id == 0 { // VSETIVLI指令，AVL来自uimm[4:0]
		avl = (ir >> 15) & 0x1f
	} else { // VSETVLI指令，AVL来自rs1寄存器
		avl = vmst.Core.Regs[rs1id]
	}

	// 确定新的向量长度(vl)。
	var new_vl uint32
	if rs1id == 0 && rdid == 0 {
		// 特殊别名 `vsetvl x0, x0, vtype`: vl被设置为VLMAX。
		new_vl = vlmax
	} else {
		// 否则，vl取min(AVL, VLMAX)。
		new_vl = avl
		if new_vl > vlmax {
			new_vl = vlmax
		}
	}
	vmst.Core.Vl = new_vl

	// 将计算出的vl值写入目标寄存器rd。
	if rdid != 0 {
		vmst.Core.Regs[rdid] = new_vl
	}

	return 0 // 成功
}

// handleOPIVX 函数处理 RISC-V 向量扩展中的 OPIVX 编码指令，
// 这些是向量-标量整数算术指令。
// ir: 32位的指令字。
// 返回: 成功时返回0，发生陷阱时返回非0值。
func (vmst *VmState) handleOPIVX(ir uint32) int32 {
	// 从指令中解码字段
	funct6 := ir >> 26
	vm := (ir >> 25) & 1
	vd := (ir >> 7) & 0x1f
	rs1id := (ir >> 15) & 0x1f // 源整数寄存器索引
	vs2 := (ir >> 20) & 0x1f

	// 获取当前选择的元素宽度(SEW)
	sew_val := (vmst.Core.Vtype >> 2) & 0x7
	sew_bytes := uint32(1 << sew_val)

	// 当前实现只支持最高32位的元素宽度
	if sew_bytes > 4 {
		return CAUSE_ILLEGAL_INSTRUCTION
	}

	// 从整数寄存器文件中读取标量操作数
	op2 := vmst.Core.Regs[rs1id]

	// 遍历向量元素
	for i := vmst.Core.Vstart; i < vmst.Core.Vl; i++ {
		// 掩码处理
		if vm == 0 {
			mask_byte_index := i / 8
			mask_bit_index := i % 8
			if (vmst.Core.Vregs[mask_byte_index] & (1 << mask_bit_index)) == 0 {
				continue
			}
		}

		// 获取元素地址
		addr2 := vmst.get_velement_addr(vs2, i, sew_bytes)
		addr_dest := vmst.get_velement_addr(vd, i, sew_bytes)

		var op1, result uint32

		// 根据funct6执行具体操作
		switch funct6 {
		case FUNCT6_VADD: // VADD.VX: 向量-标量加法
			switch sew_bytes {
			case 1:
				op1 = uint32(vmst.Core.Vregs[addr2])
				result = op1 + op2
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				result = op1 + op2
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				result = op1 + op2
				binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
			}
		case FUNCT6_VSUB: // VSUB.VX: 向量-标量减法 (vs2 - rs1)
			switch sew_bytes {
			case 1:
				op1 = uint32(vmst.Core.Vregs[addr2])
				result = op1 - op2
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				result = op1 - op2
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				result = op1 - op2
				binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
			}
		case FUNCT6_VAND: // VAND.VX: 向量-标量按位与
			switch sew_bytes {
			case 1:
				op1 = uint32(vmst.Core.Vregs[addr2])
				result = op1 & op2
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				result = op1 & op2
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				result = op1 & op2
				binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
			}
		case FUNCT6_VOR: // VOR.VX: 向量-标量按位或
			switch sew_bytes {
			case 1:
				op1 = uint32(vmst.Core.Vregs[addr2])
				result = op1 | op2
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				result = op1 | op2
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				result = op1 | op2
				binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
			}
		case FUNCT6_VXOR: // VXOR.VX: 向量-标量按位异或
			switch sew_bytes {
			case 1:
				op1 = uint32(vmst.Core.Vregs[addr2])
				result = op1 ^ op2
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				result = op1 ^ op2
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				result = op1 ^ op2
				binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
			}
		case FUNCT6_VSLL: // VSLL.VX: 向量-标量逻辑左移
			var shamt uint32
			switch sew_bytes {
			case 1:
				op1 = uint32(vmst.Core.Vregs[addr2])
				shamt = op2 & 0x7 // 移位量取rs1的低3位
				result = op1 << shamt
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				shamt = op2 & 0xf // 移位量取rs1的低4位
				result = op1 << shamt
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				shamt = op2 & 0x1f // 移位量取rs1的低5位
				result = op1 << shamt
				binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
			}
		case FUNCT6_VSRL: // VSRL.VX: 向量-标量逻辑右移
			var shamt uint32
			switch sew_bytes {
			case 1:
				op1 = uint32(vmst.Core.Vregs[addr2])
				shamt = op2 & 0x7
				result = uint32(byte(op1) >> shamt)
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				shamt = op2 & 0xf
				result = uint32(uint16(op1) >> shamt)
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				shamt = op2 & 0x1f
				result = op1 >> shamt
				binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
			}
		case FUNCT6_VSRA: // VSRA.VX: 向量-标量算术右移
			var shamt uint32
			switch sew_bytes {
			case 1:
				op1 = uint32(vmst.Core.Vregs[addr2])
				shamt = op2 & 0x7
				result = uint32(int8(op1) >> shamt)
				vmst.Core.Vregs[addr_dest] = byte(result)
			case 2:
				op1 = uint32(binary.LittleEndian.Uint16(vmst.Core.Vregs[addr2:]))
				shamt = op2 & 0xf
				result = uint32(int16(op1) >> shamt)
				binary.LittleEndian.PutUint16(vmst.Core.Vregs[addr_dest:], uint16(result))
			case 4:
				op1 = binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
				shamt = op2 & 0x1f
				result = uint32(int32(op1) >> shamt)
				binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], result)
			}
		default:
			return CAUSE_ILLEGAL_INSTRUCTION // 不支持的funct6
		}
	}

	vmst.Core.Vstart = 0 // 重置vstart
	return 0
}

// handleVFPOPIVV 函数处理向量-向量浮点算术指令 (OPFVV)。
// ir: 32位的指令字。
// 返回: 成功时返回0，发生陷阱时返回非0值。
func (vmst *VmState) handleVFPOPIVV(ir uint32) int32 {
	// 解码指令字段
	funct6 := ir >> 26
	vm := (ir >> 25) & 1
	vd := (ir >> 7) & 0x1f
	vs1 := (ir >> 15) & 0x1f
	vs2 := (ir >> 20) & 0x1f

	// 获取元素宽度
	sew_val := (vmst.Core.Vtype >> 2) & 0x7
	sew_bytes := uint32(1 << sew_val)

	// RISC-V 'V' 标准扩展中的标准浮点操作仅为 SEW=32 (单精度) 定义。
	if sew_bytes != 4 {
		return CAUSE_ILLEGAL_INSTRUCTION // 如果SEW不是32位，则为非法指令
	}

	// 遍历向量元素
	for i := vmst.Core.Vstart; i < vmst.Core.Vl; i++ {
		// 掩码处理
		if vm == 0 {
			mask_byte_index := i / 8
			mask_bit_index := i % 8
			if (vmst.Core.Vregs[mask_byte_index] & (1 << mask_bit_index)) == 0 {
				continue
			}
		}

		// 获取操作数和目标地址
		addr1 := vmst.get_velement_addr(vs1, i, sew_bytes)
		addr2 := vmst.get_velement_addr(vs2, i, sew_bytes)
		addr_dest := vmst.get_velement_addr(vd, i, sew_bytes)

		// 读取32位浮点操作数
		op1_bits := binary.LittleEndian.Uint32(vmst.Core.Vregs[addr1:])
		op2_bits := binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])

		// 将位模式转换为float32
		f1 := math.Float32frombits(op1_bits)
		f2 := math.Float32frombits(op2_bits)
		var result float32

		// 执行浮点运算
		switch funct6 {
		case FUNCT6_VFADD: // VFADD.VV: 向量-向量浮点加法
			result = f1 + f2
		case FUNCT6_VFSUB: // VFSUB.VV: 向量-向量浮点减法
			result = f1 - f2
		case FUNCT6_VFMUL: // VFMUL.VV: 向量-向量浮点乘法
			result = f1 * f2
		case FUNCT6_VFDIV: // VFDIV.VV: 向量-向量浮点除法
			result = f1 / f2
		default:
			return CAUSE_ILLEGAL_INSTRUCTION // 不支持的 funct6
		}

		// 将结果转换回位模式并存入目标寄存器
		binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], math.Float32bits(result))
	}

	vmst.Core.Vstart = 0 // 重置vstart
	return 0
}

// handleVFPOPIVF 函数处理向量-标量浮点算术指令 (OPFVF)。
// ir: 32位的指令字。
// 返回: 成功时返回0，发生陷阱时返回非0值。
func (vmst *VmState) handleVFPOPIVF(ir uint32) int32 {
	// 解码指令字段
	funct6 := ir >> 26
	vm := (ir >> 25) & 1
	vd := (ir >> 7) & 0x1f
	frs1 := (ir >> 15) & 0x1f // 源浮点寄存器索引
	vs2 := (ir >> 20) & 0x1f

	// 获取元素宽度
	sew_val := (vmst.Core.Vtype >> 2) & 0x7
	sew_bytes := uint32(1 << sew_val)

	// 只支持单精度 (32位) 浮点数
	if sew_bytes != 4 {
		return CAUSE_ILLEGAL_INSTRUCTION
	}

	// 从浮点寄存器文件中读取标量操作数
	f1 := math.Float32frombits(vmst.Core.Fregs[frs1])

	// 遍历向量元素
	for i := vmst.Core.Vstart; i < vmst.Core.Vl; i++ {
		// 掩码处理
		if vm == 0 {
			mask_byte_index := i / 8
			mask_bit_index := i % 8
			if (vmst.Core.Vregs[mask_byte_index] & (1 << mask_bit_index)) == 0 {
				continue
			}
		}

		// 获取地址
		addr2 := vmst.get_velement_addr(vs2, i, sew_bytes)
		addr_dest := vmst.get_velement_addr(vd, i, sew_bytes)

		// 读取向量元素操作数
		op2_bits := binary.LittleEndian.Uint32(vmst.Core.Vregs[addr2:])
		f2 := math.Float32frombits(op2_bits) // 向量元素
		var result float32

		// 执行浮点运算
		switch funct6 {
		case FUNCT6_VFADD: // VFADD.VF: 向量-标量浮点加法 (f2 + f1)
			result = f2 + f1
		case FUNCT6_VFSUB: // VFSUB.VF: 向量-标量浮点减法 (f2 - f1)
			result = f2 - f1
		case FUNCT6_VFRSUB: // VFRSUB.VF: 向量-标量浮点减法 (f1 - f2)
			result = f1 - f2
		case FUNCT6_VFMUL: // VFMUL.VF: 向量-标量浮点乘法 (f2 * f1)
			result = f2 * f1
		case FUNCT6_VFDIV: // VFDIV.VF: 向量-标量浮点除法 (f2 / f1)
			result = f2 / f1
		case FUNCT6_VFRDIV: // VFRDIV.VF: 向量-标量浮点除法  (f1 / f2)
			result = f1 / f2
		default:
			return CAUSE_ILLEGAL_INSTRUCTION // 不支持的 funct6
		}

		// 存储结果
		binary.LittleEndian.PutUint32(vmst.Core.Vregs[addr_dest:], math.Float32bits(result))
	}

	vmst.Core.Vstart = 0 // 重置vstart
	return 0
}
