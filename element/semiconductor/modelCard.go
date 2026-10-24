// modelCard.go：ModelCard 在 semiconductor 包的别名（m2，c2；公共结构在 element 包，避免循环）。
package semiconductor

import "circuit/element"

// ModelCard 是 element.ModelCard 的别名，便于器件代码按包内习惯使用。
type ModelCard = element.ModelCard
