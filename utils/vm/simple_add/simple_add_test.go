package simple_add

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

func TestRunSimpleAddELF(t *testing.T) {
	// 1. 构建 ELF 文件
	cmd := exec.Command("make")
	cmd.Dir = "."
	err := cmd.Run()
	if err != nil {
		t.Fatalf("构建 ELF 文件失败: %v", err)
	}
	defer exec.Command("make", "clean").Run()

	// 2. 初始化模拟器
	v_m := vm.NewVmState()

	// 3. 读取 ELF 文件
	elfData, err := os.ReadFile("main.elf")
	if err != nil {
		t.Fatalf("读取 ELF 文件失败: %v", err)
	}

	file, err := elf.NewFile(bytes.NewReader(elfData))
	if err != nil {
		t.Fatalf("解析 ELF 文件失败: %v", err)
	}
	defer file.Close()

	// 4. 将 ELF 段加载到模拟器内存
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

	// 5. 设置 PC 到入口点并运行模拟器
	v_m.SetProgramCounter(uint32(file.Entry))

	// 运行足够多的周期以完成程序。由于程序以无限循环结束，因此预计会出现 UVM32_ERR_HUNG。
	_, evt := v_m.Run(100000)
	if evt.Typ != vm.VmEvtTypErr || evt.Err.Errcode != vm.VmErrHung {
		t.Fatalf("模拟器在意外的状态下停止: Evt=%v, Err=%v", evt.Typ, evt.Err.Errcode)
	}

	// 6. 验证结果
	// 在我们的 simple_add 程序中，结果存储在地址 0x80001000
	result_addr_ram := uint32(0x80001000)
	result_offset := result_addr_ram - vm.VmRamImageOffSet
	final_val := binary.LittleEndian.Uint32(v_m.GetMemory()[result_offset:])

	expected_val := uint32(15)
	if final_val != expected_val {
		t.Errorf("测试失败：期望值 %d, 得到 %d", expected_val, final_val)
	}
}
