// modelCard_test.go：ParseModelCard 单测（m2，c2）。
package load

import (
	"strings"
	"testing"
)

// TestParseModelCard 覆盖合法 NMOS/DIODE 卡、跨行与未知参数负例。
func TestParseModelCard(t *testing.T) {
	nmos, err := ParseModelCard(`.model NCH NMOS (LEVEL=1 VTO=0.7 KP=120e-6 TNOM=300.15)`)
	if err != nil {
		t.Fatalf("解析 NMOS 失败: %v", err)
	}
	if nmos.Name != "NCH" || nmos.Type != "NMOS" {
		t.Errorf("name/type=%s/%s", nmos.Name, nmos.Type)
	}
	if got := nmos.Param("VTO", 0); got != 0.7 {
		t.Errorf("VTO=%v，期望 0.7", got)
	}
	if got := nmos.Param("TNOM", 0); got != 300.15 {
		t.Errorf("TNOM=%v，期望 300.15", got)
	}

	diode, err := ParseModelCard(".model DMOD D\n+ IS=1e-14 N=1.0 TNOM=300")
	if err != nil {
		t.Fatalf("解析 DIODE 失败: %v", err)
	}
	if got := diode.Param("IS", 0); got != 1e-14 {
		t.Errorf("IS=%v", got)
	}

	if _, err := ParseModelCard(".model BAD NMOS (BOGUS=1)"); err == nil || !strings.Contains(err.Error(), "BOGUS") {
		t.Errorf("未知参数应报错且含参数名，实际 %v", err)
	}
	if _, err := ParseModelCard("not a model card"); err == nil {
		t.Errorf("非 .model 卡应报错")
	}
}
