// spec_test.go 校验 spec.go 的参数表与 specTable.md 同源、单位换算正确，并把要求覆盖的 11 个型号钉死。
package ttl

import (
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

// requiredModels 是本表必须覆盖的 11 个型号；缺一个即判失败。
var requiredModels = []string{
	"74LS00", "74LS04", "74LS08", "74LS32", "74LS74", "74LS86",
	"74LS138", "74LS164", "74LS245", "74LS373", "74LS574",
}

// specTableRow 是 specTable.md 参数表里的一行：8 个数值列按 TpdLH, TpdHL, IIH, IIL, IOH, IOL, VOH, VOL 排列。
type specTableRow struct {
	model  string
	fields [8]string
	source string
}

// unverifiedCell 是表里表示"未取证"的固定写法。
const unverifiedCell = "（未取证）"

// parseSpecTable 解析 specTable.md 的参数表；表头、分隔行与其它表格按型号列过滤掉。
func parseSpecTable(t *testing.T) []specTableRow {
	t.Helper()
	raw, err := os.ReadFile("specTable.md")
	if err != nil {
		t.Fatalf("读取 specTable.md 失败: %v", err)
	}
	var rows []specTableRow
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := strings.Split(trimmed, "|")
		if len(cells) != 12 {
			continue
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if !isModelName(cells[1]) {
			continue
		}
		var row specTableRow
		row.model = cells[1]
		copy(row.fields[:], cells[2:10])
		row.source = cells[10]
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		t.Fatal("specTable.md 里没有解析到任何型号行")
	}
	return rows
}

// isModelName 判断单元格是否为 74LSxx 形式的型号名。
func isModelName(cell string) bool {
	if !strings.HasPrefix(cell, "74LS") || len(cell) <= len("74LS") {
		return false
	}
	for _, r := range cell[len("74LS"):] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isVerified 判断某行是否至少有一个数值列已取证。
func (r specTableRow) isVerified() bool {
	for _, cell := range r.fields {
		if cell != unverifiedCell {
			return true
		}
	}
	return false
}

// parseCellValue 把表内单元格换算成 SI 值：后缀 u = 1e-6 A，m = 1e-3 A，无后缀为原值。
func parseCellValue(t *testing.T, cell string) (float64, bool) {
	t.Helper()
	if cell == unverifiedCell {
		return 0, false
	}
	scale := 1.0
	number := cell
	switch {
	case strings.HasSuffix(cell, "u"):
		scale, number = 1e-6, strings.TrimSuffix(cell, "u")
	case strings.HasSuffix(cell, "m"):
		scale, number = 1e-3, strings.TrimSuffix(cell, "m")
	}
	value, err := strconv.ParseFloat(number, 64)
	if err != nil {
		t.Fatalf("单元格 %q 不是可解析的数值: %v", cell, err)
	}
	return value * scale, true
}

// TestRequiredModelsResolve 判据 1：11 个型号 SpecOf 全部命中，已取证行的 Source 非空。
func TestRequiredModelsResolve(t *testing.T) {
	rows := parseSpecTable(t)
	byModel := make(map[string]specTableRow, len(rows))
	for _, row := range rows {
		byModel[row.model] = row
	}
	for _, model := range requiredModels {
		row, inTable := byModel[model]
		if !inTable {
			t.Fatalf("specTable.md 缺少型号 %s", model)
		}
		spec, ok := SpecOf(model)
		if !ok {
			t.Fatalf("SpecOf(%s) 未命中", model)
		}
		if row.isVerified() && spec.Source == "" {
			t.Fatalf("型号 %s 已有取证数值但 Source 为空", model)
		}
		if !row.isVerified() && spec.Source != "" {
			t.Fatalf("型号 %s 整行未取证却给了 Source=%q", model, spec.Source)
		}
	}
}

// TestUnknownModelReturnsFalse 判据 2：未知型号返回 false，且不得回退到默认值。
func TestUnknownModelReturnsFalse(t *testing.T) {
	for _, model := range []string{"", "74LS999", "74ls00", "7400", "74LS00 ", "SN74LS00", "74HC00"} {
		spec, ok := SpecOf(model)
		if ok {
			t.Fatalf("SpecOf(%q) 应返回 false，实际命中 %s", model, spec.Model)
		}
		if spec != (DeviceSpec{}) {
			t.Fatalf("SpecOf(%q) 未命中却返回了非零值: %+v", model, spec)
		}
	}
}

// TestTableAndCodeShareSameModels 判据 3：md 型号集合 == 代码可命中型号集合。
func TestTableAndCodeShareSameModels(t *testing.T) {
	rows := parseSpecTable(t)
	inTable := make(map[string]bool, len(rows))
	for _, row := range rows {
		if inTable[row.model] {
			t.Fatalf("specTable.md 里型号 %s 重复出现", row.model)
		}
		inTable[row.model] = true
	}
	inCode := make(map[string]bool, len(deviceSpecs))
	for _, spec := range deviceSpecs {
		if inCode[spec.Model] {
			t.Fatalf("deviceSpecs 里型号 %s 重复", spec.Model)
		}
		inCode[spec.Model] = true
	}
	for model := range inTable {
		if !inCode[model] {
			t.Fatalf("型号 %s 在 specTable.md 里但代码表中没有", model)
		}
	}
	for model := range inCode {
		if !inTable[model] {
			t.Fatalf("型号 %s 在代码表里但 specTable.md 中没有", model)
		}
	}
	if len(inTable) != len(requiredModels) {
		t.Fatalf("型号数量 = %d，要求 %d", len(inTable), len(requiredModels))
	}
}

// TestNanoSecondConversion 判据 4：Tpd 秒值 == 表内 ns 值 × 1e-9（容差 1e-12）。
func TestNanoSecondConversion(t *testing.T) {
	const tolerance = 1e-12
	for _, row := range parseSpecTable(t) {
		spec, ok := SpecOf(row.model)
		if !ok {
			t.Fatalf("SpecOf(%s) 未命中", row.model)
		}
		for index, got := range []float64{spec.TpdLH, spec.TpdHL} {
			cell := row.fields[index]
			nanoseconds, verified := parseCellValue(t, cell)
			if !verified {
				if got != 0 {
					t.Fatalf("%s 的 Tpd 单元格为未取证，代码却给了 %g", row.model, got)
				}
				continue
			}
			want := nanoseconds * 1e-9
			if math.Abs(got-want) > tolerance {
				t.Fatalf("%s 第 %d 个 Tpd: 代码 %g s，表内 %s ns 换算应为 %g s", row.model, index+1, got, cell, want)
			}
		}
	}
}

// TestUnverifiedCellsStayZero 判据 5：表里标未取证的字段，代码侧必须是零值（禁止填推测值）。
func TestUnverifiedCellsStayZero(t *testing.T) {
	const tolerance = 1e-12
	for _, row := range parseSpecTable(t) {
		spec, ok := SpecOf(row.model)
		if !ok {
			t.Fatalf("SpecOf(%s) 未命中", row.model)
		}
		got := [8]float64{spec.TpdLH, spec.TpdHL, spec.IIH, spec.IIL, spec.IOH, spec.IOL, spec.VOH, spec.VOL}
		for index, cell := range row.fields {
			value, verified := parseCellValue(t, cell)
			if !verified {
				if got[index] != 0 {
					t.Fatalf("%s 第 %d 列标未取证，代码却为 %g", row.model, index+1, got[index])
				}
				continue
			}
			want := value
			if index < 2 {
				want = value * 1e-9
			}
			if math.Abs(got[index]-want) > tolerance {
				t.Fatalf("%s 第 %d 列: 代码 %g，表内 %s 应为 %g", row.model, index+1, got[index], cell, want)
			}
		}
	}
}
