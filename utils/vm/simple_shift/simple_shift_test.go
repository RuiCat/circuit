package simple_shift

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

func TestRunSimpleShiftELF(t *testing.T) {
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

	// 验证 SLLI
	expected_slli := uint32(0x1230)
	result_slli := binary.LittleEndian.Uint32(v_m.GetMemory()[offset:])
	if result_slli != expected_slli {
		t.Errorf("SLLI 测试失败: 期望值 %#x, 得到 %#x", expected_slli, result_slli)
	}

	// 验证 SRLI
	expected_srli := uint32(0x12)
	result_srli := binary.LittleEndian.Uint32(v_m.GetMemory()[offset+4:])
	if result_srli != expected_srli {
		t.Errorf("SRLI 测试失败: 期望值 %#x, 得到 %#x", expected_srli, result_srli)
	}

	// 验证 SRAI
	expected_srai := uint32(0xFFFFFFFB) // -5
	result_srai := binary.LittleEndian.Uint32(v_m.GetMemory()[offset+8:])
	if result_srai != expected_srai {
		t.Errorf("SRAI 测试失败: 期望值 %#x, 得到 %#x", expected_srai, result_srai)
	}

	// 验证 SLL
	expected_sll := uint32(0x123 << 5)
	result_sll := binary.LittleEndian.Uint32(v_m.GetMemory()[offset+12:])
	if result_sll != expected_sll {
		t.Errorf("SLL 测试失败: 期望值 %#x, 得到 %#x", expected_sll, result_sll)
	}

	// 验证 SRL
	expected_srl := uint32(0x123 >> 5)
	result_srl := binary.LittleEndian.Uint32(v_m.GetMemory()[offset+16:])
	if result_srl != expected_srl {
		t.Errorf("SRL 测试失败: 期望值 %#x, 得到 %#x", expected_srl, result_srl)
	}

	// 验证 SRA
	expected_sra := uint32(0xFFFFFFFD) // -3
	result_sra := binary.LittleEndian.Uint32(v_m.GetMemory()[offset+20:])
	if int32(result_sra) != -3 {
		t.Errorf("SRA 测试失败: 期望值 %#x, 得到 %#x", expected_sra, result_sra)
	}
}
