package ast

import (
	"strings"
	"testing"
)

// TestParseBasicElements 验证基本元件(R/V/Q)的网表解析，检查解析后的元件数量、类型和ID。
func TestParseBasicElements(t *testing.T) {
	input := `R1 [1,0] [1000]
V1 [1,0] [0,0,0,0,5]
Q1 [2,3,0] [false,100]`

	tree, err := NewParseTree(strings.NewReader(input))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(tree.ElementNodes) != 3 {
		t.Fatalf("期望 3 个元件，得到 %d", len(tree.ElementNodes))
	}
	if tree.ElementNodes[0].Type != "R" || tree.ElementNodes[0].ID != "1" {
		t.Fatalf("第一个元件异常: %s%s", tree.ElementNodes[0].Type, tree.ElementNodes[0].ID)
	}
}

// TestParseSubCircuitBasic 验证子电路(.subckt/.ends)定义解析，检查端口名和内部元件。
func TestParseSubCircuitBasic(t *testing.T) {
	input := `.subckt inv in out
R1 [in,out] [100]
Q1 [out,in,0] [false,100]
.ends inv`

	tree, err := NewParseTree(strings.NewReader(input))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(tree.SubCircuitDefs) != 1 {
		t.Fatalf("期望 1 个子电路定义，得到 %d", len(tree.SubCircuitDefs))
	}
	def := tree.SubCircuitDefs[0]
	if def.Name != "inv" {
		t.Fatalf("子电路名称期望 'inv'，得到 '%s'", def.Name)
	}
	if len(def.Ports) != 2 {
		t.Fatalf("期望 2 个端口，得到 %d", len(def.Ports))
	}
	if def.Ports[0].Value != "in" || def.Ports[1].Value != "out" {
		t.Fatalf("端口异常: %v", def.Ports)
	}
	if len(def.Elements) != 2 {
		t.Fatalf("期望 2 个元件，得到 %d", len(def.Elements))
	}
}

// TestParseMultipleSubCircuits 验证多个子电路与顶层元件混合解析，检查子电路和顶层元件的正确分离。
func TestParseMultipleSubCircuits(t *testing.T) {
	input := `.subckt inv in out
R1 [in,out] [100]
.ends inv

R1 [1,0] [1000]

.subckt buf in out
X1 [in,mid] inv
X2 [mid,out] inv
.ends buf`

	tree, err := NewParseTree(strings.NewReader(input))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(tree.SubCircuitDefs) != 2 {
		t.Fatalf("期望 2 个子电路定义，得到 %d", len(tree.SubCircuitDefs))
	}
	if len(tree.ElementNodes) != 1 {
		t.Fatalf("期望 1 个顶层元件，得到 %d", len(tree.ElementNodes))
	}
	if tree.ElementNodes[0].Type != "R" {
		t.Fatalf("顶层元件类型期望 'R'，得到 '%s'", tree.ElementNodes[0].Type)
	}
}

// TestParseNestedSubCircuits 验证嵌套子电路(.subckt内嵌.subckt)的解析，检查内外层层级结构。
func TestParseNestedSubCircuits(t *testing.T) {
	input := `.subckt outer a b
.subckt inner c d
R1 [c,d] [100]
.ends inner
R2 [a,b] [200]
.ends outer`

	tree, err := NewParseTree(strings.NewReader(input))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(tree.SubCircuitDefs) != 1 {
		t.Fatalf("期望 1 个顶层子电路定义，得到 %d", len(tree.SubCircuitDefs))
	}
	outer := tree.SubCircuitDefs[0]
	if outer.Name != "outer" {
		t.Fatalf("外层子电路名称期望 'outer'，得到 '%s'", outer.Name)
	}
	if len(outer.Defs) != 1 {
		t.Fatalf("期望外层有 1 个嵌套子电路，得到 %d", len(outer.Defs))
	}
	inner := outer.Defs[0]
	if inner.Name != "inner" {
		t.Fatalf("内层子电路名称期望 'inner'，得到 '%s'", inner.Name)
	}
	if len(outer.Elements) != 1 {
		t.Fatalf("期望外层有 1 个元件，得到 %d", len(outer.Elements))
	}
	if len(inner.Elements) != 1 {
		t.Fatalf("期望内层有 1 个元件，得到 %d", len(inner.Elements))
	}
}

// TestParseSubCircuitInstance 验证子电路实例(X元件)的解析，检查元件类型、ID、引脚和引用名称。
func TestParseSubCircuitInstance(t *testing.T) {
	input := `.subckt inv in out
R1 [in,out] [100]
.ends inv

X1 [1,0] inv`

	tree, err := NewParseTree(strings.NewReader(input))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(tree.ElementNodes) != 1 {
		t.Fatalf("期望 1 个顶层元件，得到 %d", len(tree.ElementNodes))
	}
	x := tree.ElementNodes[0]
	if x.Type != "X" || x.ID != "1" {
		t.Fatalf("子电路实例异常: %s%s", x.Type, x.ID)
	}
	if len(x.Pins) != 2 {
		t.Fatalf("子电路实例期望 2 个引脚，得到 %d", len(x.Pins))
	}
	if len(x.Values) != 1 {
		t.Fatalf("子电路实例期望 1 个值（子电路名称），得到 %d", len(x.Values))
	}
	if x.Values[0].Value != "inv" {
		t.Fatalf("子电路实例名称期望 'inv'，得到 '%s'", x.Values[0].Value)
	}
}

// TestParseSubCircuitWithValues 验证.value变量设置与子电路内%变量引用解析，检查变量键值和IsVar标志。
func TestParseSubCircuitWithValues(t *testing.T) {
	input := `.value R 1000

.subckt amp in out
R1 [in,out] [%R]
.ends amp`

	tree, err := NewParseTree(strings.NewReader(input))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(tree.ValueNodes) != 1 {
		t.Fatalf("期望 1 个值设置，得到 %d", len(tree.ValueNodes))
	}
	if tree.ValueNodes["R"] != "1000" {
		t.Fatalf("值设置异常: %v", tree.ValueNodes)
	}
	if len(tree.SubCircuitDefs) != 1 {
		t.Fatalf("期望 1 个子电路定义，得到 %d", len(tree.SubCircuitDefs))
	}
	amp := tree.SubCircuitDefs[0]
	if len(amp.Elements) != 1 {
		t.Fatalf("期望 1 个元件，得到 %d", len(amp.Elements))
	}
	if len(amp.Elements[0].Values) != 1 {
		t.Fatalf("期望 1 个值，得到 %d", len(amp.Elements[0].Values))
	}
	if !amp.Elements[0].Values[0].IsVar {
		t.Fatalf("期望值 R 是变量")
	}
}

// TestParseSubCircuitWithComment 验证行注释(#和//)和块注释(/* */)的正确忽视。
func TestParseSubCircuitWithComment(t *testing.T) {
	input := `.subckt test in out
# 这是一个注释
R1 [in,out] [100]
// 这也是注释
.ends test`

	tree, err := NewParseTree(strings.NewReader(input))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(tree.SubCircuitDefs) != 1 {
		t.Fatalf("期望 1 个子电路定义，得到 %d", len(tree.SubCircuitDefs))
	}
	def := tree.SubCircuitDefs[0]
	if len(def.Elements) != 1 {
		t.Fatalf("期望 1 个元件，得到 %d", len(def.Elements))
	}
	if def.Name != "test" {
		t.Fatalf("子电路名称期望 'test'，得到 '%s'", def.Name)
	}
}

// TestParseUnmatchedEnds 验证孤立的.ends指令解析报错，检查错误信息是否包含"意外的.ends"。
func TestParseUnmatchedEnds(t *testing.T) {
	input := `.ends`
	_, err := NewParseTree(strings.NewReader(input))
	if err == nil {
		t.Fatal("期望错误，但解析成功")
	}
	if !strings.Contains(err.Error(), "意外的 .ends") {
		t.Fatalf("错误信息异常: %v", err)
	}
}

// TestParseMissingSubCktName 验证.subckt缺少名称时的解析报错，检查错误信息是否包含"缺少名称"。
func TestParseMissingSubCktName(t *testing.T) {
	input := `.subckt`
	_, err := NewParseTree(strings.NewReader(input))
	if err == nil {
		t.Fatal("期望错误，但解析成功")
	}
	if !strings.Contains(err.Error(), "缺少名称") {
		t.Fatalf("错误信息异常: %v", err)
	}
}

// TestParseSubCircuitMissingInstanceName 验证子电路实例X元件缺少子电路名称时的解析报错。
func TestParseSubCircuitMissingInstanceName(t *testing.T) {
	input := `.subckt test a b
X1 [a,b]
.ends test`
	_, err := NewParseTree(strings.NewReader(input))
	if err == nil {
		t.Fatal("期望错误，但解析成功")
	}
	if !strings.Contains(err.Error(), "缺少子电路名称") {
		t.Fatalf("错误信息异常: %v", err)
	}
}

// TestParseEmptyNetlist 验证空网表的解析，检查返回的解析树是否为空。
func TestParseEmptyNetlist(t *testing.T) {
	tree, err := NewParseTree(strings.NewReader(""))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(tree.ElementNodes) != 0 || len(tree.SubCircuitDefs) != 0 {
		t.Fatal("期望空解析树")
	}
}

// TestParseCaseInsensitiveSubCkt 验证.subckt/.ends关键字大小写不敏感，检查.SUBCKT和.ENDS是否能正确识别。
func TestParseCaseInsensitiveSubCkt(t *testing.T) {
	input := `.SUBCKT test a b
R1 [a,b] [100]
.ENDS test`

	tree, err := NewParseTree(strings.NewReader(input))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(tree.SubCircuitDefs) != 1 {
		t.Fatalf("期望 1 个子电路定义，得到 %d", len(tree.SubCircuitDefs))
	}
}

// TestParseEmptySubCircuit 验证空子电路(无端口无元件)的解析，检查名称、端口和元件数量。
func TestParseEmptySubCircuit(t *testing.T) {
	input := `.subckt empty
.ends empty`

	tree, err := NewParseTree(strings.NewReader(input))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(tree.SubCircuitDefs) != 1 {
		t.Fatalf("期望 1 个子电路定义，得到 %d", len(tree.SubCircuitDefs))
	}
	def := tree.SubCircuitDefs[0]
	if def.Name != "empty" {
		t.Fatalf("名称期望 'empty'，得到 '%s'", def.Name)
	}
	if len(def.Ports) != 0 {
		t.Fatalf("期望 0 个端口，得到 %d", len(def.Ports))
	}
	if len(def.Elements) != 0 {
		t.Fatalf("期望 0 个元件，得到 %d", len(def.Elements))
	}
}

// TestParseMultipleNestedLevels 验证多个平级嵌套子电路的解析，检查嵌套数量和名称。
func TestParseMultipleNestedLevels(t *testing.T) {
	input := `.subckt a x y
.subckt b z
R1 [z,x] [100]
.ends b
.subckt c w
R2 [w,y] [200]
.ends c
.ends a`

	tree, err := NewParseTree(strings.NewReader(input))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(tree.SubCircuitDefs) != 1 {
		t.Fatalf("期望 1 个顶层子电路，得到 %d", len(tree.SubCircuitDefs))
	}
	a := tree.SubCircuitDefs[0]
	if len(a.Defs) != 2 {
		t.Fatalf("期望 a 有 2 个嵌套子电路，得到 %d", len(a.Defs))
	}
	if a.Defs[0].Name != "b" || a.Defs[1].Name != "c" {
		t.Fatalf("嵌套子电路名称异常: %s, %s", a.Defs[0].Name, a.Defs[1].Name)
	}
}

// TestParseSubCircuitWithXInstance 验证包含子电路实例(X元件)的子电路定义解析，检查嵌套引用关系。
func TestParseSubCircuitWithXInstance(t *testing.T) {
	input := `.subckt nand a b out
R1 [vcc,out] [1000]
Q1 [a,out,0] [false,100]
Q2 [b,out,0] [false,100]
.ends nand

.subckt half_adder a b sum cout
X1 [a,b,sum] nand
X2 [a,cout,sum] nand
.ends half_adder`

	tree, err := NewParseTree(strings.NewReader(input))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(tree.SubCircuitDefs) != 2 {
		t.Fatalf("期望 2 个子电路定义，得到 %d", len(tree.SubCircuitDefs))
	}

	nand := tree.SubCircuitDefs[0]
	if nand.Name != "nand" {
		t.Fatalf("名称期望 'nand'，得到 '%s'", nand.Name)
	}
	if len(nand.Elements) != 3 {
		t.Fatalf("nand 期望 3 个元件，得到 %d", len(nand.Elements))
	}

	half := tree.SubCircuitDefs[1]
	if half.Name != "half_adder" {
		t.Fatalf("名称期望 'half_adder'，得到 '%s'", half.Name)
	}
	if len(half.Elements) != 2 {
		t.Fatalf("half_adder 期望 2 个元件，得到 %d", len(half.Elements))
	}
	for i, elem := range half.Elements {
		if elem.Type != "X" {
			t.Fatalf("half_adder 元件 %d 类型期望 'X'，得到 '%s'", i, elem.Type)
		}
		if len(elem.Values) != 1 || elem.Values[0].Value != "nand" {
			t.Fatalf("half_adder 元件 %d 子电路名称期望 'nand'，得到 '%v'", i, elem.Values)
		}
	}
}

// TestParseQuotedStringValues 验证双引号字符串字面量被整体解析（不切碎）。
// 回归：SplitTokens 原先把 "642C0916" 切成 "、642、C0916、" 四个碎片，
// 导致 RAM 元件的 init_hex/mem_hex 参数错位。
func TestParseQuotedStringValues(t *testing.T) {
	input := `RAM1 [A0,A1,D0,D1,WE,Q0,Q1] [256, "642C0916"]`

	tree, err := NewParseTree(strings.NewReader(input))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(tree.ElementNodes) != 1 {
		t.Fatalf("期望 1 个元件，得到 %d", len(tree.ElementNodes))
	}
	elem := tree.ElementNodes[0]
	if elem.Type != "RAM" {
		t.Fatalf("元件类型期望 'RAM'，得到 '%s'", elem.Type)
	}
	if len(elem.Values) != 2 {
		t.Fatalf("期望 2 个值，得到 %d: %v", len(elem.Values), elem.Values)
	}
	if elem.Values[0].Value != "256" {
		t.Fatalf("值 0 期望 '256'，得到 '%s'", elem.Values[0].Value)
	}
	if elem.Values[1].Value != `"642C0916"` {
		t.Fatalf("值 1 期望完整字符串字面量 %q，得到 %q", `"642C0916"`, elem.Values[1].Value)
	}
	if got := elem.Values[1].ParseString(""); got != "642C0916" {
		t.Fatalf("ParseString 期望剥离引号得 '642C0916'，得到 '%s'", got)
	}

	// 未闭合字符串字面量应报错
	if _, err := NewParseTree(strings.NewReader(`RAM1 [A0,A1,D0,D1,WE,Q0,Q1] [256, "642C0916]`)); err == nil {
		t.Fatal("期望未闭合字符串字面量报错，但解析成功")
	}
}
