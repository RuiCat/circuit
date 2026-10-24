// modelCard.go：SPICE .model 卡的公共结构（m2，c2）。
package element

// ModelCard 是解析后的 .model 卡；Params 的键统一大写。
type ModelCard struct {
	Name   string
	Type   string
	Params map[string]float64
}

// Param 返回参数值；缺省返回 def。
func (m ModelCard) Param(key string, def float64) float64 {
	if m.Params == nil {
		return def
	}
	if v, ok := m.Params[key]; ok {
		return v
	}
	return def
}
