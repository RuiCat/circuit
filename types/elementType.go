package types

import "fmt"

// Init 初始化
func (t ElementType) Init(value *ElementBase) ElementFace {
	if et, ok := slementTypeString[t]; ok {
		return et.ElementConfig.Init(value)
	}
	return nil
}

// InitValue 初始化元件值
func (t ElementType) InitValue() Value {
	if et, ok := slementTypeString[t]; ok {
		return et.ElementConfig.InitValue()
	}
	return nil
}

// 获取引脚数量
func (t ElementType) GetPostCount() int {
	if et, ok := slementTypeString[t]; ok {
		return et.ElementConfig.GetPostCount()
	}
	return 0
}

// 电路元件类型常量定义
// TypeUnknown 未知类型
const (
	TypeUnknown ElementType = iota // 未知类型
)

// slementTypeString 元件映射
var slementTypeString = map[ElementType]struct {
	Name          string
	ElementConfig ElementConfig
}{
	TypeUnknown: {Name: "Unknown", ElementConfig: nil},
}

// String 返回元件类型的字符串表示
func (t ElementType) String() string {
	if et, ok := slementTypeString[t]; ok {
		return et.Name
	}
	return "Unknown"
}

var mapName = map[string]ElementType{
	"Unknown": TypeUnknown,
}

// GetNameType 通过名称获取类型
func GetNameType(name string) ElementType {
	return mapName[name]
}

// ElementRegister 注册元件类型
func ElementRegister(et ElementType, name string, config ElementConfig) {
	if _, ok := slementTypeString[et]; ok {
		panic(fmt.Errorf("指定元件类型已经注册: %s:%d", name, et))
	}
	mapName[name] = et
	slementTypeString[et] = struct {
		Name          string
		ElementConfig ElementConfig
	}{
		Name:          name,
		ElementConfig: config,
	}
}
