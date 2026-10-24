// meas.go：SPICE `.meas` 最小实现（m3，c3）。
package analysis

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// MeasResult 是一条 .meas 的结果。
type MeasResult struct {
	Name  string
	Value float64
}

// RunMeas 解析 `.meas <tran|dc> <name> <func> <signal> [AT=<t>]` 并对给定信号求值。
// 支持 AVG / MAX / MIN / PP / FIND（FIND 用 AT 找最近时间点）。
func RunMeas(text string, signals map[string][]float64, times []float64) ([]MeasResult, error) {
	out := make([]MeasResult, 0)
	for _, raw := range strings.Split(text, "\n") {
		f := strings.Fields(strings.TrimSpace(raw))
		if len(f) == 0 || !strings.EqualFold(f[0], ".meas") {
			continue
		}
		if len(f) < 5 {
			return nil, fmt.Errorf(".meas 行参数不足: %q", raw)
		}
		name, fn, sig := f[2], strings.ToUpper(f[3]), f[4]
		data, ok := signals[sig]
		if !ok || len(data) == 0 {
			return nil, fmt.Errorf(".meas %s: 未知信号 %q", name, sig)
		}
		value, err := measValue(fn, data, times, f[5:])
		if err != nil {
			return nil, fmt.Errorf(".meas %s: %w", name, err)
		}
		out = append(out, MeasResult{Name: name, Value: value})
	}
	return out, nil
}

// measValue 对一条 .meas 的取值函数求值。
func measValue(fn string, data []float64, times []float64, opts []string) (float64, error) {
	switch fn {
	case "AVG":
		return mean(data), nil
	case "MAX":
		return maxOf(data), nil
	case "MIN":
		return minOf(data), nil
	case "PP":
		return maxOf(data) - minOf(data), nil
	case "FIND":
		at, err := findOpt(opts, "AT")
		if err != nil {
			return 0, err
		}
		return nearestValue(data, times, at), nil
	default:
		return 0, fmt.Errorf("不支持的 .meas 函数 %q", fn)
	}
}

// findOpt 从 KEY=VAL 选项里取数值。
func findOpt(opts []string, key string) (float64, error) {
	for _, o := range opts {
		kv := strings.SplitN(o, "=", 2)
		if len(kv) == 2 && strings.EqualFold(kv[0], key) {
			v, err := strconv.ParseFloat(kv[1], 64)
			if err != nil {
				return 0, fmt.Errorf("%s 不是数值: %q", key, kv[1])
			}
			return v, nil
		}
	}
	return 0, fmt.Errorf("FIND 缺 %s= 选项", key)
}

// nearestValue 返回时间上离 at 最近的信号值；times 缺失时按索引近似。
func nearestValue(data, times []float64, at float64) float64 {
	if len(times) != len(data) || len(times) == 0 {
		idx := int(at)
		if idx < 0 {
			idx = 0
		}
		if idx >= len(data) {
			idx = len(data) - 1
		}
		return data[idx]
	}
	best, bestDiff := 0, abs(times[0]-at)
	for i := 1; i < len(times); i++ {
		if d := abs(times[i] - at); d < bestDiff {
			best, bestDiff = i, d
		}
	}
	return data[best]
}

func mean(data []float64) float64 {
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

func maxOf(data []float64) float64 {
	m := data[0]
	for _, v := range data[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func minOf(data []float64) float64 {
	m := data[0]
	for _, v := range data[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// sortedKeys 是测试辅助：确定性遍历信号名。
func sortedKeys(m map[string][]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
