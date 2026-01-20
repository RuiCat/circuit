package logic

import (
	"bytes"
	"circuit/utils/vm"
	"debug/elf"
	"encoding/binary"
	"io"
	"os"
	"os/exec"
	"testing"
)

func TestRunSimpleLogicELF(t *testing.T) {
	cmd := exec.Command("make")
	cmd.Dir = "."
	err := cmd.Run()
	if err != nil {
		t.Fatalf("构建 ELF 文件失败: %v", err)
	}
	defer exec.Command("make", "clean").Run()

	v_m := vm.NewVmState()
	elfData, err := os.ReadFile("main.elf")
	if err != nil {
		t.Fatalf("读取 ELF 文件失败: %v", err)
	}

	file, err := elf.NewFile(bytes.NewReader(elfData))
	if err != nil {
		t.Fatalf("解析 ELF 文件失败: %v", err)
	}
	defer file.Close()

	for _, prog := range file.Progs {
		if prog.Type == elf.PT_LOAD {
			if prog.Paddr < vm.VmRamImageOffSet {
				t.Fatalf("程序段地址 (0x%x) 无效", prog.Paddr)
			}
			memOffset := prog.Paddr - vm.VmRamImageOffSet
			if memOffset+prog.Filesz > uint64(len(v_m.GetMemory())) {
				t.Fatalf("程序段对于模拟器内存来说太大了")
			}
			data, err := io.ReadAll(prog.Open())
			if err != nil {
				t.Fatalf("读取程序段失败: %v", err)
			}
			copy(v_m.GetMemory()[memOffset:], data)
		}
	}

	v_m.SetProgramCounter(uint32(file.Entry))
	_, evt := v_m.Run(100000)
	if evt.Typ != vm.VmEvtTypErr || evt.Err.Errcode != vm.VmErrHung {
		t.Fatalf("模拟器在意外的状态下停止: Evt=%v, Err=%v", evt.Typ, evt.Err.Errcode)
	}

	base_addr := uint32(0x80001000)
	offset := base_addr - vm.VmRamImageOffSet

	// 验证 ANDI
	expected_andi := uint32(8)
	result_andi := binary.LittleEndian.Uint32(v_m.GetMemory()[offset:])
	if result_andi != expected_andi {
		t.Errorf("ANDI 测试失败: 期望值 %d, 得到 %d", expected_andi, result_andi)
	}

	// 验证 ORI
	expected_ori := uint32(14)
	result_ori := binary.LittleEndian.Uint32(v_m.GetMemory()[offset+4:])
	if result_ori != expected_ori {
		t.Errorf("ORI 测试失败: 期望值 %d, 得到 %d", expected_ori, result_ori)
	}

	// 验证 XORI
	expected_xori := uint32(10)
	result_xori := binary.LittleEndian.Uint32(v_m.GetMemory()[offset+8:])
	if result_xori != expected_xori {
		t.Errorf("XORI 测试失败: 期望值 %d, 得到 %d", expected_xori, result_xori)
	}

	// 验证 AND
	expected_and := uint32(8)
	result_and := binary.LittleEndian.Uint32(v_m.GetMemory()[offset+12:])
	if result_and != expected_and {
		t.Errorf("AND 测试失败: 期望值 %d, 得到 %d", expected_and, result_and)
	}

	// 验证 OR
	expected_or := uint32(14)
	result_or := binary.LittleEndian.Uint32(v_m.GetMemory()[offset+16:])
	if result_or != expected_or {
		t.Errorf("OR 测试失败: 期望值 %d, 得到 %d", expected_or, result_or)
	}

	// 验证 XOR
	expected_xor := uint32(6)
	result_xor := binary.LittleEndian.Uint32(v_m.GetMemory()[offset+20:])
	if result_xor != expected_xor {
		t.Errorf("XOR 测试失败: 期望值 %d, 得到 %d", expected_xor, result_xor)
	}
}
