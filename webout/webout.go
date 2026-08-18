// Package webout 提供自包含 HTML 波形页输出，供 cmd 批处理与 MCP 服务器共用。
package webout

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Meta 记录单网页头部展示的仿真元信息。
type Meta struct {
	Title      string  `json:"title"`
	Elements   int     `json:"elements"`
	Nodes      int     `json:"nodes"`
	Steps      int     `json:"steps"`
	TargetTime float64 `json:"targetTime"`
	FinalTime  float64 `json:"finalTime"`
}

// webPageData 是嵌入 HTML 页面的完整数据载荷。
// Units/Kinds 与 Series 一一对应（不含 time 列），Kinds 取值 "voltage" / "current"。
type pageData struct {
	Title   string      `json:"title"`
	Headers []string    `json:"headers"`
	Time    []float64   `json:"time"`
	Series  [][]float64 `json:"series"`
	Units   []string    `json:"units"`
	Kinds   []string    `json:"kinds"`
	Meta    Meta        `json:"meta"`
}

// WriteHTML 把仿真结果序列化为自包含单网页并写入 w。
// rows 每行为 [时间, 列1, 列2, ...]，与 headers 的列一一对应；units/kinds 描述除时间外的每列。
func WriteHTML(w io.Writer, headers []string, rows [][]float64, units, kinds []string, meta Meta) error {
	if len(rows) == 0 {
		return fmt.Errorf("无仿真数据可输出")
	}
	n := len(headers) - 1
	if n < 1 {
		return fmt.Errorf("无输出数据列")
	}
	if len(units) != n || len(kinds) != n {
		return fmt.Errorf("units/kinds 长度 (%d/%d) 与数据列数 %d 不匹配", len(units), len(kinds), n)
	}
	time := make([]float64, len(rows))
	series := make([][]float64, n)
	for c := range series {
		series[c] = make([]float64, len(rows))
	}
	for r, row := range rows {
		if len(row) < 1+n {
			return fmt.Errorf("第 %d 行数据长度不符（期望 %d 列，实际 %d）", r, 1+n, len(row))
		}
		time[r] = row[0]
		for c := 0; c < n; c++ {
			series[c][r] = row[c+1]
		}
	}

	data := pageData{Title: meta.Title, Headers: headers, Time: time, Series: series, Units: units, Kinds: kinds, Meta: meta}
	js, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("编码波形数据失败: %w", err)
	}
	// encoding/json 默认会把 < > & 转义为 \u003c 等，因此不会产生 </script> 提前闭合。
	page := strings.Replace(htmlPageTemplate, "__DATA__", string(js), 1)
	if _, err := io.WriteString(w, page); err != nil {
		return err
	}
	return nil
}
