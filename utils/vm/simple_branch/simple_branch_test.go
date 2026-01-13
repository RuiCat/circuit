package simple_branch

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

func TestRunSimpleBranchELF(t *testing.T) {
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
		{"BEQ_taken", 0, 1},
		{"BEQ_not_taken", 4, 2},
		{"BNE_taken", 8, 3},
		{"BNE_not_taken", 12, 4},
		{"BLT_taken", 16, 5},
		{"BLT_not_taken", 20, 6},
		{"BGE_taken", 24, 7},
		{"BGE_not_taken", 28, 8},
		{"BLTU_taken", 32, 9},
		{"BLTU_not_taken", 36, 10},
		{"BGEU_taken", 40, 11},
		{"BGEU_not_taken", 44, 12},
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
