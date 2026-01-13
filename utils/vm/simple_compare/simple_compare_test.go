package simple_compare

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

func TestRunSimpleCompareELF(t *testing.T) {
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

	tests := []struct {
		name     string
		offset   uint32
		expected uint32
	}{
		{"SLTI_true", 0, 1},
		{"SLTI_false", 4, 0},
		{"SLTIU_true", 8, 1},
		{"SLTIU_false", 12, 0},
		{"SLT_true", 16, 1},
		{"SLT_false", 20, 0},
		{"SLTU_true", 24, 1},
		{"SLTU_false", 28, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := binary.LittleEndian.Uint32(v_m.GetMemory()[offset+tt.offset:])
			if result != tt.expected {
				t.Errorf("%s 测试失败: 期望值 %d, 得到 %d", tt.name, tt.expected, result)
			}
		})
	}
}
