package base

import (
	"circuit/element"
	"circuit/load/ast"
	"circuit/mna"
	"strings"
)

// WrapperType 层级封装元件类型标识。
var WrapperType element.NodeType

// Wrapper 层级封装元件，将子电路封装为单个元件实例。
// 对外表现为一组引脚，对内持有子电路的全部元件作为子元件。
// 子元件加入全局 Nodelist 由仿真引擎统一调度，Wrapper 自身不介入仿真计算。
type Wrapper struct{ *element.Config }

// NewWrapperConfig 基于子电路定义动态创建层级封装元件的配置。
// ports: 子电路端口名称
// name:  元件标识名称，由 X 实例名决定
func NewWrapperConfig(ports []string, name string) *element.Config {
	pins := make([]element.Pin, len(ports))
	for i, p := range ports {
		pins[i] = element.Pin{Name: p, Type: element.PinBoolean}
	}
	return &element.Config{
		Name: strings.ToUpper(name),
		Pin:  pins,
		ValueInit: []any{
			"", // 0: 子电路名称
		},
		ValueName: []string{"subckt"},
	}
}

// NewWrapperElement 注册层级封装元件到全局元件列表。
// subcktName: 用于注册表中的唯一标识名，建议使用子电路原名
func NewWrapperElement(subcktName string) element.NodeType {
	typeName := strings.ToUpper(subcktName)
	upper := "X_" + typeName
	if _, exists := element.ElementListName[upper]; exists {
		return element.ElementListName[upper]
	}
	wt := element.AddElement(element.NodeType(19), &Wrapper{
		NewWrapperConfig(nil, subcktName),
	})
	element.ElementListName[upper] = wt
	return wt
}

// Base 层级封装元件的配置信息。
// elem 的 Pins 包含外部引脚，Values[0] 为子电路名称。
func (w *Wrapper) Base(elem ast.ElementNode) *element.Config {
	pins := make([]element.Pin, len(elem.Pins))
	for i, p := range elem.Pins {
		pins[i] = element.Pin{Name: p.Value, Type: element.PinBoolean}
	}
	subcktName := ""
	if len(elem.Values) > 0 {
		subcktName = elem.Values[0].Value
	}
	return &element.Config{
		Name: "X",
		Pin:  pins,
		ValueInit: []any{
			subcktName,
		},
		ValueName: []string{"subckt"},
	}
}

// Stamp 层级封装元件：子元件已加入 Nodelist 由仿真引擎统一调度，此处为空操作。
func (w *Wrapper) Stamp(m mna.Mna, t mna.Time, value element.NodeFace) {}

// DoStep 层级封装元件：子元件已加入 Nodelist 由仿真引擎统一调度，此处为空操作。
func (w *Wrapper) DoStep(m mna.Mna, t mna.Time, value element.NodeFace) {}

// init 确保 Wrapper 元件在包初始化时注册。
func init() {
	if _, ok := element.ElementListName["X"]; !ok {
		WrapperType = element.AddElement(element.NodeType(17), &Wrapper{
			&element.Config{
				Name: "X",
				Pin:  []element.Pin{},
				ValueInit: []any{
					"", // 0: 子电路名称
				},
				ValueName: []string{"subckt"},
			},
		})
		element.ElementListName["X"] = WrapperType
	} else {
		WrapperType = element.ElementListName["X"]
	}
}
