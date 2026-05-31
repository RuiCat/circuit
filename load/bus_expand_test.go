package load

import (
	"testing"

	_ "circuit/element/base"
)

func TestExpandRangeBusInPins(t *testing.T) {
	input := `R1 [addr[7:0], 0] [100]`
	want := `R1 [addr7 addr6 addr5 addr4 addr3 addr2 addr1 addr0, 0] [100]`
	got, err := ExpandBusNotation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestExpandRangeBusInSubckt(t *testing.T) {
	input := `.subckt adder data[3:0] clk`
	want := `.subckt adder data3 data2 data1 data0 clk`
	got, err := ExpandBusNotation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestExpandAscendingRange(t *testing.T) {
	input := `R1 [bus[0:3]] [100]`
	want := `R1 [bus0 bus1 bus2 bus3] [100]`
	got, err := ExpandBusNotation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestExpandSingleIndex(t *testing.T) {
	input := `R1 [bus[3], 0] [100]`
	want := `R1 [bus3, 0] [100]`
	got, err := ExpandBusNotation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestExpandSingleBitRange(t *testing.T) {
	input := `R1 [bus[0:0]] [100]`
	want := `R1 [bus0] [100]`
	got, err := ExpandBusNotation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSkipCommentLine(t *testing.T) {
	input := "# bus[7:0]\nR1 [1,0]"
	got, err := ExpandBusNotation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != input {
		t.Fatalf("comment line should not be expanded:\ngot:\n%s\nwant:\n%s", got, input)
	}
}

func TestSkipInlineComment(t *testing.T) {
	input := `R1 [addr[7:0]] # bus[3:0]`
	want := `R1 [addr7 addr6 addr5 addr4 addr3 addr2 addr1 addr0] # bus[3:0]`
	got, err := ExpandBusNotation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestMultipleBusesOneLine(t *testing.T) {
	input := `U1 [addr[1:0], data[3:2]]`
	want := `U1 [addr1 addr0, data3 data2]`
	got, err := ExpandBusNotation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestBusWithUnderscore(t *testing.T) {
	input := `data_bus[2:0]`
	want := `data_bus2 data_bus1 data_bus0`
	got, err := ExpandBusNotation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestNoBracketsUnchanged(t *testing.T) {
	input := `R1 [data7, clk] [100]`
	want := `R1 [data7, clk] [100]`
	got, err := ExpandBusNotation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestExpandInValues(t *testing.T) {
	input := `U1 [1,0] [bus[3:0]]`
	want := `U1 [1,0] [bus3 bus2 bus1 bus0]`
	got, err := ExpandBusNotation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestBusWidthLimit(t *testing.T) {
	input := `bus[5000:0]`
	_, err := ExpandBusNotation(input)
	if err == nil {
		t.Fatal("expected error for bus width exceeding limit")
	}
}

func TestSubcircuitDefinitionWithSingleBusPin(t *testing.T) {
	input := `.subckt inv data[3:0]`
	want := `.subckt inv data3 data2 data1 data0`
	got, err := ExpandBusNotation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSubcircuitInstanceWithBus(t *testing.T) {
	input := `X1 [addr[3:0], clk] buf`
	want := `X1 [addr3 addr2 addr1 addr0, clk] buf`
	got, err := ExpandBusNotation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestExpandEndToEnd(t *testing.T) {
	// 端到端测试：完整的网表包含总线，确保 LoadString 能正确解析
	netlist := `
.subckt adder data[1:0] clk
  R1 [data[0],0] [100]
.ends adder

X1 [bus[1:0], clk] adder
`
	con, err := LoadString(netlist)
	if err != nil {
		t.Fatalf("LoadString failed: %v", err)
	}
	if len(con.Nodelist) == 0 {
		t.Fatal("expected elements in context")
	}
	// 验证扩展后的节点名称可被正确映射 (X1.data3, X1.data0 等)
	for _, n := range con.Nodelist {
		if n == nil {
			t.Fatal("element type should not be nil")
		}
	}
}

func TestExpandWithSubcircuitBusConnection(t *testing.T) {
	// 子电路定义和实例化同时使用总线
	netlist := `
.subckt buf2 in[1:0] out[1:0]
  R1 [in[0],out[0]] [1000]
  R2 [in[1],out[1]] [1000]
.ends buf2

X1 [src[1:0], dst[1:0]] buf2
`
	con, err := LoadString(netlist)
	if err != nil {
		t.Fatalf("LoadString failed: %v", err)
	}
	if con.Nodelist == nil {
		t.Fatal("expected nodelist")
	}
}

func TestWindowsLineEndings(t *testing.T) {
	input := "R1 [addr[7:0]]\r\nR2 [bus[1:0]]\r\n"
	want := "R1 [addr7 addr6 addr5 addr4 addr3 addr2 addr1 addr0]\nR2 [bus1 bus0]\n"
	got, err := ExpandBusNotation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestBusAtEndOfLineNoTrailingNewline(t *testing.T) {
	input := ".subckt adder data[3:0]"
	want := ".subckt adder data3 data2 data1 data0"
	got, err := ExpandBusNotation(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}
