package load

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "circuit/element/register"
)

// TestLoadRejectsCrossDomainNode 电气引脚与气路引脚混接同一节点必须被拒绝。
func TestLoadRejectsCrossDomainNode(t *testing.T) {
	_, err := LoadString(`
V1 [1, -1] [0, 0, 0, 0, 10]
PS1 [1, -1] [101325]
`)
	if err == nil {
		t.Fatal("期望电气/气路混接同一节点被拒绝，但加载成功")
	}
	if !strings.Contains(err.Error(), "跨域混接") {
		t.Fatalf("错误信息未说明跨域原因: %v", err)
	}
}

// TestLoadRejectsHydraulicElectricMix 液压引脚与电气引脚混接同一节点必须被拒绝。
func TestLoadRejectsHydraulicElectricMix(t *testing.T) {
	_, err := LoadString(`
HP1 [1, -1] [10e6]
R1 [1, -1] [1000]
`)
	if err == nil {
		t.Fatal("期望液压/电气混接同一节点被拒绝，但加载成功")
	}
	if !strings.Contains(err.Error(), "跨域混接") {
		t.Fatalf("错误信息未说明跨域原因: %v", err)
	}
}

// TestLoadAllowsSameDomainMixedPinTypes 电气域内（弱电+布尔）混接允许（逻辑门网表模式）。
func TestLoadAllowsSameDomainMixedPinTypes(t *testing.T) {
	_, err := LoadString(`
V1 [1, -1] [0, 0, 0, 0, 5]
U1 [1, 2, 3] [1, 5]
R1 [3, -1] [10000]
`)
	if err != nil {
		t.Fatalf("电气域内弱电/布尔引脚混接应允许: %v", err)
	}
}

// TestLoadAllowsSharedGroundAcrossDomains 全局地节点 -1 可被电气/气路/液压各域共用（转换器模式）。
func TestLoadAllowsSharedGroundAcrossDomains(t *testing.T) {
	_, err := LoadString(`
V1 [1, -1] [0, 0, 0, 0, 10]
EP1 [1, -1, 3, -1] [100000, 0]
PR1 [3, 4] [1e8]
PC1 [4] [1e-8]
`)
	if err != nil {
		t.Fatalf("各域共享 -1 地节点应允许: %v", err)
	}
}

// TestLoadAllowsConverterSideDomains EP/EH 转换器两侧分属不同域但连接不同节点，允许。
func TestLoadAllowsConverterSideDomains(t *testing.T) {
	_, err := LoadString(`
V1 [1, -1] [0, 0, 0, 0, 10]
EH1 [1, -1, 3, -1] [1e6, 0]
HA1 [3] [1e-8]
`)
	if err != nil {
		t.Fatalf("转换器两侧不同域但不同节点应允许: %v", err)
	}
}

// TestLoadAllowsMixedDomainSubcircuit 子电路（X 封装）内部跨域元件不误伤，且经 X 端口混接仍被拒绝。
func TestLoadAllowsMixedDomainSubcircuit(t *testing.T) {
	_, err := LoadString(`
.subckt ep a b p
V1 [a, b] [0, 0, 0, 0, 10]
EP1 [a, b, p, b] [100000, 0]
.ends
X1 [1, -1, 3] ep
PC1 [3] [1e-8]
`)
	if err != nil {
		t.Fatalf("子电路内部跨域元件应允许: %v", err)
	}

	// 外部电气元件经 X 端口接入内部气路端口节点，仍应被拒绝（内部展开子元件参与校验）。
	_, err = LoadString(`
.subckt pn a b
PS1 [a, b] [101325]
.ends
X1 [1, -1] pn
V1 [1, -1] [0, 0, 0, 0, 10]
`)
	if err == nil {
		t.Fatal("期望经 X 端口跨域混接被拒绝，但加载成功")
	}
	if !strings.Contains(err.Error(), "跨域混接") {
		t.Fatalf("错误信息未说明跨域原因: %v", err)
	}
}

// TestExampleNetlistsNotAffected 现有示例网表不应被跨域校验误伤。
func TestExampleNetlistsNotAffected(t *testing.T) {
	matches, err := filepath.Glob("../cmd/circuits/*.net")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Skip("未找到示例网表目录")
	}
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", path, err)
		}
		_, err = LoadString(string(data))
		if err != nil && strings.Contains(err.Error(), "跨域混接") {
			t.Errorf("示例网表 %s 被跨域校验误伤: %v", path, err)
		}
	}
}
