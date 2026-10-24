// modelCard.go：.model 卡解析（m2，c2）。
package load

import (
	"fmt"
	"strconv"
	"strings"

	"circuit/element"
)

// ParseModelCard 解析 `.model NAME TYPE (K=V ...)`（支持跨行与 * 注释）。
// 未知参数返回 error 且错误文本包含参数名；不支持的类型返回 error。
func ParseModelCard(text string) (element.ModelCard, error) {
	card := element.ModelCard{Params: map[string]float64{}}
	var fields []string
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "*") {
			continue
		}
		fields = append(fields, strings.Fields(line)...)
	}
	if len(fields) < 3 || strings.ToLower(fields[0]) != ".model" {
		return card, fmt.Errorf("model card 必须以 .model NAME TYPE 开头")
	}
	card.Name = fields[1]
	typ := strings.ToUpper(strings.Trim(fields[2], "()"))
	allowed := allowedModelParams(typ)
	if allowed == nil {
		return card, fmt.Errorf("不支持的 model 类型 %q", typ)
	}
	card.Type = typ
	for _, f := range fields[3:] {
		f = strings.Trim(f, "()")
		if f == "" || f == "+" {
			continue
		}
		kv := strings.SplitN(f, "=", 2)
		if len(kv) != 2 {
			return card, fmt.Errorf("参数 %q 不是 KEY=VALUE 形式", f)
		}
		key := strings.ToUpper(kv[0])
		val, err := strconv.ParseFloat(kv[1], 64)
		if err != nil {
			return card, fmt.Errorf("参数 %s=%q 不是数值", key, kv[1])
		}
		if _, ok := allowed[key]; !ok {
			return card, fmt.Errorf("unknown model parameter %q in %s", key, card.Name)
		}
		card.Params[key] = val
	}
	return card, nil
}

// allowedModelParams 返回某类型允许的参数集合；未知类型返回 nil。
func allowedModelParams(typ string) map[string]struct{} {
	params := []string{"LEVEL", "TNOM", "VTO", "KP", "GAMMA", "PHI", "LAMBDA", "TOX", "NSUB", "U0", "RD", "RS"}
	switch typ {
	case "NMOS", "PMOS":
	case "D", "DIODE":
		params = []string{"IS", "N", "RS", "TNOM", "CJO", "VJ", "M", "BV", "IBV", "TT"}
	default:
		return nil
	}
	set := make(map[string]struct{}, len(params))
	for _, p := range params {
		set[p] = struct{}{}
	}
	return set
}
