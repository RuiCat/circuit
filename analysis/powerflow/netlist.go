package powerflow

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseNetlist 解析潮流网表(B/BR 语法,标幺值),与 MCP circuit_run_powerflow 的 buses/branches 语义一致:
//
//	# 注释(整行以 # 开头)
//	B1 slack V=1.05                      — 平衡母线:V 必给,Theta 可选(度)
//	B2 pv   V=1.0 P=0.5 Qmin=-0.3 Qmax=0.3 — 发电机母线:V、P 必给,无功限幅可选
//	B3 pq   P=-1.0 Q=-0.5                — 负荷母线:P、Q 必给
//	BR1 1-2 R=0.01 X=0.1                 — 支路:From-To,R/X 必给且不可同时为 0,B/Tap 可选
//
// Theta 单位为度(内部换算为弧度);参数键不区分大小写;其余字段缺省与 MCP 工具一致
// (V 缺省 0 由求解器按平启动/母线类型补 1.0,Qmin/Qmax 缺省 0 = 不限制)。
func ParseNetlist(text string) (*Input, error) {
	in := &Input{}
	for lineNo, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("第 %d 行: %q 缺少参数", lineNo+1, fields[0])
		}
		name := strings.ToUpper(fields[0])
		switch {
		case strings.HasPrefix(name, "BR") && isDigits(name[2:]):
			br, err := parseBranchLine(name[2:], fields[1:], lineNo+1)
			if err != nil {
				return nil, err
			}
			in.Branches = append(in.Branches, *br)
		case strings.HasPrefix(name, "B") && isDigits(name[1:]):
			b, err := parseBusLine(name[1:], fields[1:], lineNo+1)
			if err != nil {
				return nil, err
			}
			in.Buses = append(in.Buses, *b)
		default:
			return nil, fmt.Errorf("第 %d 行: 无法识别的网表行 %q(潮流网表仅支持 B/BR 行与 # 注释)", lineNo+1, fields[0])
		}
	}
	if len(in.Buses) == 0 {
		return nil, fmt.Errorf("潮流网表没有母线(B 行)")
	}
	return in, nil
}

// parseBusLine 解析一条母线行:B<id> slack|pv|pq [V=..] [Theta=..度] [P=..] [Q=..] [Qmin=..] [Qmax=..]
func parseBusLine(idStr string, fields []string, lineNo int) (*Bus, error) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return nil, fmt.Errorf("第 %d 行: 母线编号 %q 非法", lineNo, idStr)
	}
	b := &Bus{ID: id}
	switch strings.ToLower(fields[0]) {
	case "slack":
		b.Type = BusSlack
	case "pv":
		b.Type = BusPV
	case "pq":
		b.Type = BusPQ
	default:
		return nil, fmt.Errorf("第 %d 行: 母线 %d 类型必须为 slack/pv/pq,实际 %q", lineNo, id, fields[0])
	}
	kv, err := parseKV(fields[1:], lineNo)
	if err != nil {
		return nil, err
	}
	for k, v := range kv {
		switch k {
		case "v":
			b.V = v
		case "theta":
			b.Theta = v * math.Pi / 180 // 度 → 弧度
		case "p":
			b.P = v
		case "q":
			b.Q = v
		case "qmin":
			b.Qmin = v
		case "qmax":
			b.Qmax = v
		default:
			return nil, fmt.Errorf("第 %d 行: 母线 %d 不支持的参数 %q(支持 v/theta/p/q/qmin/qmax)", lineNo, id, k)
		}
	}
	return b, nil
}

// parseBranchLine 解析一条支路行:BR<id> From-To [R=..] [X=..] [B=..] [Tap=..]
func parseBranchLine(idStr string, fields []string, lineNo int) (*Branch, error) {
	if _, err := strconv.Atoi(idStr); err != nil {
		return nil, fmt.Errorf("第 %d 行: 支路编号 %q 非法", lineNo, idStr)
	}
	ends := strings.SplitN(fields[0], "-", 2)
	if len(ends) != 2 {
		return nil, fmt.Errorf("第 %d 行: 支路端点应为 From-To 形式,实际 %q", lineNo, fields[0])
	}
	from, err1 := strconv.Atoi(strings.TrimSpace(ends[0]))
	to, err2 := strconv.Atoi(strings.TrimSpace(ends[1]))
	if err1 != nil || err2 != nil {
		return nil, fmt.Errorf("第 %d 行: 支路端点 %q 非法", lineNo, fields[0])
	}
	br := &Branch{From: from, To: to}
	kv, err := parseKV(fields[1:], lineNo)
	if err != nil {
		return nil, err
	}
	for k, v := range kv {
		switch k {
		case "r":
			br.R = v
		case "x":
			br.X = v
		case "b":
			br.B = v
		case "tap":
			br.Tap = v
		default:
			return nil, fmt.Errorf("第 %d 行: 支路 %d-%d 不支持的参数 %q(支持 r/x/b/tap)", lineNo, from, to, k)
		}
	}
	if br.R == 0 && br.X == 0 {
		return nil, fmt.Errorf("第 %d 行: 支路 %d-%d 阻抗 R/X 不能同时为 0", lineNo, from, to)
	}
	return br, nil
}

// parseKV 解析 key=value 参数列表(键不区分大小写)。
func parseKV(fields []string, lineNo int) (map[string]float64, error) {
	kv := make(map[string]float64, len(fields))
	for _, f := range fields {
		eq := strings.IndexByte(f, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("第 %d 行: 参数 %q 应为 key=value 形式", lineNo, f)
		}
		key := strings.ToLower(strings.TrimSpace(f[:eq]))
		val, err := strconv.ParseFloat(strings.TrimSpace(f[eq+1:]), 64)
		if err != nil {
			return nil, fmt.Errorf("第 %d 行: 参数 %s 的值 %q 非法", lineNo, key, f[eq+1:])
		}
		kv[key] = val
	}
	return kv, nil
}

// isDigits 判断字符串是否全部由数字组成(空串返回 false)。
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
