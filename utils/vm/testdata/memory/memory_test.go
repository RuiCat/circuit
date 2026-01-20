package memory

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

func TestRunSimpleMemoryELF(t *testing.T) {
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
	// 增加运行周期以确保有足够的时间执行所有内存操作
	_, evt := v_m.Run(200000)
	if evt.Typ != vm.VmEvtTypErr || evt.Err.Errcode != vm.VmErrHung {
		t.Fatalf("模拟器在意外的状态下停止: Evt=%v, Err=%v", evt.Typ, evt.Err.Errcode)
	}

	base_addr := uint32(0x80001100)
	offset := base_addr - vm.VmRamImageOffSet

	// --- 验证加载结果 ---
	loadTests := []struct {
		name     string
		offset   uint32
		expected uint32
	}{
		{"LW", 0, 0x5678ABCD},
		{"LH", 4, 0xFFFFDEFA},
		{"LHU", 8, 0x0000DEFA},
		{"LB", 12, 0xFFFFFF8A},
		{"LBU", 16, 0x0000008A},
	}
	for _, tt := range loadTests {
		t.Run(tt.name, func(t *testing.T) {
			result := binary.LittleEndian.Uint32(v_m.GetMemory()[offset+tt.offset:])
			if result != tt.expected {
				t.Errorf("%s 测试失败: 期望值 %#x, 得到 %#x", tt.name, tt.expected, result)
			}
		})
	}

	// --- 验证存储结果 ---
	// SB 验证
	val_sb := v_m.GetMemory()[offset+20]
	if val_sb != 0x8A {
		t.Errorf("SB 测试失败: 期望值 %#x, 得到 %#x", byte(0x8A), val_sb)
	}

	// SH 验证
	val_sh := binary.LittleEndian.Uint16(v_m.GetMemory()[offset+22:])
	if val_sh != 0xDEFA {
		t.Errorf("SH 测试失败: 期望值 %#x, 得到 %#x", uint16(0xDEFA), val_sh)
	}

	// SW 验证
	val_sw := binary.LittleEndian.Uint32(v_m.GetMemory()[offset+24:])
	if val_sw != 0x5678ABCD {
		t.Errorf("SW 测试失败: 期望值 %#x, 得到 %#x", uint32(0x5678ABCD), val_sw)
	}
}
