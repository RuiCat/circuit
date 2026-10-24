// spiceNetlist.go：SPICE 子集 → 自定义网表转换（m6，c4）。
package load

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseSpiceNetlist 把 SPICE 子集（R/C/L/V/I + .model/.param/.tran/.ac/.meas）转换为
// 现有自定义网表文本；不支持的指令/元件返回 error（非零语义由调用方 CLI 处理）。
func ParseSpiceNetlist(text string) (string, error) {
	params := map[string]string{}
	var out []string
	nodeMap := map[string]int{}
	nextNode := 1000
	nodeID := func(name string) int {
		up := strings.ToUpper(name)
		if up == "0" || up == "GND" {
			return -1
		}
		if id, ok := nodeMap[up]; ok {
			return id
		}
		if n, err := strconv.Atoi(up); err == nil {
			nodeMap[up] = n
			return n
		}
		nodeMap[up] = nextNode
		nextNode++
		return nodeMap[up]
	}

	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "*") {
			continue
		}
		if strings.HasPrefix(line, ".") {
			fields := strings.Fields(line)
			switch strings.ToLower(fields[0]) {
			case ".model":
				if _, err := ParseModelCard(line); err != nil {
					return "", err
				}
			case ".param":
				for _, kv := range fields[1:] {
					parts := strings.SplitN(kv, "=", 2)
					if len(parts) != 2 {
						return "", fmt.Errorf(".param 参数 %q 不是 NAME=VALUE", kv)
					}
					params[strings.ToUpper(parts[0])] = parts[1]
				}
			case ".tran", ".ac", ".meas", ".include":
				// 元数据/后续分析消费，转换为自定义网表时跳过。
			default:
				return "", fmt.Errorf("不支持的 SPICE 指令 %q", fields[0])
			}
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			return "", fmt.Errorf("元件行参数不足: %q", line)
		}
		name := fields[0]
		typ := strings.ToUpper(name[:1])
		n1, n2 := nodeID(fields[1]), nodeID(fields[2])
		valIdx := 3
		if strings.EqualFold(fields[3], "dc") || strings.EqualFold(fields[3], "ac") {
			valIdx = 4
			if len(fields) <= valIdx {
				return "", fmt.Errorf("%s: 缺源值", name)
			}
		}
		valStr := substituteParams(fields[valIdx], params)
		switch typ {
		case "R", "C", "L", "I":
			v, err := parseSpiceValue(valStr)
			if err != nil {
				return "", fmt.Errorf("%s: %w", name, err)
			}
			out = append(out, fmt.Sprintf("%s [%d,%d] [%g]", name, n1, n2, v))
		case "V":
			v, err := parseSpiceValue(valStr)
			if err != nil {
				return "", fmt.Errorf("%s: %w", name, err)
			}
			out = append(out, fmt.Sprintf("%s [%d,%d] [0,0,0,0,%g]", name, n1, n2, v))
		default:
			return "", fmt.Errorf("不支持的元件类型 %q（行 %q）", typ, line)
		}
	}
	if len(out) == 0 {
		return "", fmt.Errorf("SPICE 网表没有可转换的元件")
	}
	return strings.Join(out, "\n"), nil
}

// substituteParams 把 {NAME} 替换为 .param 值。
func substituteParams(s string, params map[string]string) string {
	for k, v := range params {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

// parseSpiceValue 解析 SPICE 数值后缀（k/m/u/n/p/f）。
func parseSpiceValue(s string) (float64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("空数值")
	}
	mult := 1.0
	switch s[len(s)-1] {
	case 'f':
		mult = 1e-15
	case 'p':
		mult = 1e-12
	case 'n':
		mult = 1e-9
	case 'u':
		mult = 1e-6
	case 'm':
		mult = 1e-3
	case 'k':
		mult = 1e3
	case 'g':
		mult = 1e9
	default:
		v, err := strconv.ParseFloat(s, 64)
		return v, err
	}
	v, err := strconv.ParseFloat(s[:len(s)-1], 64)
	if err != nil {
		return 0, err
	}
	return v * mult, nil
}
