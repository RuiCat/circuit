package simple_fp

import (
	"bytes"
	"circuit/utils/vm"
	"debug/elf"
	"encoding/binary"
	"io"
	"math"
	"os"
	"os/exec"
	"testing"
)

// float32Equal 比较两个float32值是否在误差范围内相等
func float32Equal(a, b float32) bool {
	if math.IsNaN(float64(a)) && math.IsNaN(float64(b)) {
		return true
	}
	const epsilon = 1e-6
	return math.Abs(float64(a-b)) < epsilon
}

func TestRunSimpleFPELF(t *testing.T) {
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

	// 查找 .bss section 来确定 result_area 的地址
	var resultAreaAddr uint64
	for _, sec := range file.Sections {
		if sec.Name == ".bss" {
			resultAreaAddr = sec.Addr
			break
		}
	}
	if resultAreaAddr == 0 {
		t.Fatalf("未找到 .bss section")
	}

	for _, prog := range file.Progs {
		if prog.Type == elf.PT_LOAD {
			if prog.Paddr < vm.VmRamImageOffSet {
				t.Fatalf("程序段地址 (0x%x) 无效", prog.Paddr)
			}
			memOffset := prog.Paddr - uint64(vm.VmRamImageOffSet)
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
	_, evt := v_m.Run(200000)
	if evt.Typ != vm.VmEvtTypErr || evt.Err.Errcode != vm.VmErrHung {
		t.Fatalf("模拟器在意外的状态下停止: Evt=%v, Err=%v", evt.Typ, evt.Err.Errcode)
	}

	offset := uint32(resultAreaAddr - vm.VmRamImageOffSet)
	mem := v_m.GetMemory()

	// --- 验证结果 ---
	tests := []struct {
		name        string
		offset      uint32
		isFloat     bool
		expectedF32 float32
		expectedU32 uint32
	}{
		// 原有测试
		{"FADD", 0, true, 4.71, 0},
		{"FSUB", 4, true, 1.57, 0},
		{"FMUL", 8, true, 4.9298, 0},
		{"FDIV", 12, true, 2.0, 0},
		{"FSQRT", 16, true, 1.7720045, 0},
		{"FCVT.S.W", 20, true, 42.0, 0},
		{"FCVT.W.S", 24, false, 0, 3},
		{"FMV.W.X", 28, false, 0, 42},
		{"FMV.X.W", 32, false, 0, math.Float32bits(3.14)},
		{"FEQ (false)", 36, false, 0, 0},
		{"FEQ (true)", 40, false, 0, 1},
		// 新增测试
		{"FMIN.S", 44, true, 1.57, 0},
		{"FMAX.S", 48, true, 3.14, 0},
		{"FMIN.S (-0.0)", 52, true, -0.0, 0},
		{"FCLASS.S (pos normal)", 56, false, 0, 1 << 6},
		{"FCLASS.S (neg zero)", 60, false, 0, 1 << 3},
		{"FCVT.S.D(FCVT.D.S)", 64, true, 3.14, 0},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			resultU32 := binary.LittleEndian.Uint32(mem[offset+tt.offset:])
			if tt.isFloat {
				resultF32 := math.Float32frombits(resultU32)
				// 特殊处理 -0.0 的比较
				if tt.expectedF32 == -0.0 && resultF32 != -0.0 {
					t.Errorf("%s 测试失败: 期望值 %f, 得到 %f (bit: %#x)", tt.name, tt.expectedF32, resultF32, resultU32)
				} else if !float32Equal(resultF32, tt.expectedF32) {
					t.Errorf("%s 测试失败: 期望值 %f, 得到 %f", tt.name, tt.expectedF32, resultF32)
				}
			} else {
				if resultU32 != tt.expectedU32 {
					t.Errorf("%s 测试失败: 期望值 %#x, 得到 %#x", tt.name, tt.expectedU32, resultU32)
				}
			}
		})
	}
}
