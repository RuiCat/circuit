// Package mcp 提供 circuit 仿真引擎的 MCP(Model Context Protocol) 服务器实现。
// 包含会话管理、异步仿真任务、元件检视与修改、结果导出等完整工具集。
package mcp

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ---------- 工具参数提取辅助 ----------

// argStr 读取字符串参数。
func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

// argFloat 读取浮点参数（兼容 JSON 数字解码为 float64 / int 的情况）。
func argFloat(args map[string]any, key string) (float64, bool) {
	switch v := args[key].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

// argInt 读取整数参数。
func argInt(args map[string]any, key string) (int, bool) {
	switch v := args[key].(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

// argBool 读取布尔参数。
func argBool(args map[string]any, key string) (bool, bool) {
	v, ok := args[key].(bool)
	return v, ok
}

// argStrSlice 读取字符串数组参数。
func argStrSlice(args map[string]any, key string) []string {
	v, ok := args[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(v))
	for _, e := range v {
		if s, ok := e.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// argFloatSlice 读取浮点数组参数。
func argFloatSlice(args map[string]any, key string) []float64 {
	v, ok := args[key].([]any)
	if !ok {
		return nil
	}
	out := make([]float64, 0, len(v))
	for _, e := range v {
		if f, ok := toFloat(e); ok {
			out = append(out, f)
		}
	}
	return out
}

// argIntSlice 读取整数数组参数。
func argIntSlice(args map[string]any, key string) []int {
	v, ok := args[key].([]any)
	if !ok {
		return nil
	}
	out := make([]int, 0, len(v))
	for _, e := range v {
		if f, ok := toFloat(e); ok {
			out = append(out, int(f))
		}
	}
	return out
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	}
	return 0, false
}

// ---------- 数值与格式辅助 ----------

// formatFloat 以最短科学记数法格式化浮点数。
func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'e', -1, 64)
}

// sortedKeys 返回 map 的排序键列表。
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// fmtID 格式化会话/任务 ID 校验错误。
func errMissing(field string) error {
	return fmt.Errorf("缺少必填参数 %q", field)
}
