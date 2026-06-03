// Package register 统一注册所有基础元件类型。
// 导入此包后，所有元件类型自动通过 init() 注册到 element.ElementList 全局表。
package register

import (
	_ "circuit/element/analog"
	_ "circuit/element/controlled"
	_ "circuit/element/electromechanical"
	_ "circuit/element/hierarchical"
	_ "circuit/element/hydraulic"
	_ "circuit/element/interactive"
	_ "circuit/element/logic"
	_ "circuit/element/magnetic"
	_ "circuit/element/passive"
	_ "circuit/element/pneumatic"
	_ "circuit/element/semiconductor"
	_ "circuit/element/sensor"
	_ "circuit/element/source"
	_ "circuit/element/switch"
)
