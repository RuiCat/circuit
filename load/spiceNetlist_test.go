// spiceNetlist_test.go：SPICE 子集转换单测（m6，c4）。
package load

import (
	"strings"
	"testing"
)

// TestSpiceNetlist 验证分组器/元件/参数后缀/.model/.tran 转换后可加载。
func TestSpiceNetlist(t *testing.T) {
	spice := `
* divider
.param VDD=5
V1 in 0 DC {VDD}
R1 in out 1k
R2 out 0 1k
.model NMOS NMOS (LEVEL=1 VTO=0.7)
.tran 1n 1u
.meas tran vout AVG V(out)
`
	netlist, err := ParseSpiceNetlist(spice)
	if err != nil {
		t.Fatalf("ParseSpiceNetlist 失败: %v", err)
	}
	for _, want := range []string{
		"V1 [1000,-1] [0,0,0,0,5]",
		"R1 [1000,1001] [1000]",
		"R2 [1001,-1] [1000]",
	} {
		if !strings.Contains(netlist, want) {
			t.Errorf("转换结果缺 %q\n%s", want, netlist)
		}
	}
	if _, err := LoadString(netlist); err != nil {
		t.Errorf("转换结果不可加载: %v", err)
	}
	if _, err := ParseSpiceNetlist("X1 a b subcktname"); err == nil {
		t.Errorf("不支持的元件应报错")
	}
}
