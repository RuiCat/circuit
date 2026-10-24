// modelCard_test.go：ModelCard 参数求值单测（m2，c2）。
package semiconductor

import "testing"

// TestModelCardParam 验证参数读取与缺省。
func TestModelCardParam(t *testing.T) {
	card := ModelCard{Name: "NCH", Type: "NMOS", Params: map[string]float64{"VTO": 0.7, "KP": 120e-6}}
	if got := card.Param("VTO", 0); got != 0.7 {
		t.Errorf("VTO=%v", got)
	}
	if got := card.Param("KP", 0); got != 120e-6 {
		t.Errorf("KP=%v", got)
	}
	if got := card.Param("TOX", 1e-7); got != 1e-7 {
		t.Errorf("缺省 TOX=%v", got)
	}
	var empty ModelCard
	if got := empty.Param("VTO", 3); got != 3 {
		t.Errorf("nil Params 缺省=%v", got)
	}
}
