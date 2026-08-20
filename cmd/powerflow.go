package main

import (
	"fmt"
	"html"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"circuit/analysis/powerflow"
)

// pfMagPhase 相量 → 幅值/相位(度)。
func pfMagPhase(v complex128) (mag, phaseDeg float64) {
	return math.Hypot(real(v), imag(v)), math.Atan2(imag(v), real(v)) * 180 / math.Pi
}

// isPowerflowNetlist 判断网表是否为潮流(B/BR)格式:
// 存在以 "B<编号> slack|pv|pq" 开头的母线行,或 "BR<编号> From-To" 开头的支路行。
// 注意元件引擎也有类型 "B"(EventSwitch 开关,语法 B1 [1,0] [...]),
// 因此母线行必须第二个词元为 slack/pv/pq 才判定为潮流,避免误判普通元件网表。
func isPowerflowNetlist(text string) bool {
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		upper := strings.ToUpper(fields[0])
		switch {
		case strings.HasPrefix(upper, "BR") && isDigitsOnly(upper[2:]):
			return true
		case strings.HasPrefix(upper, "B") && isDigitsOnly(upper[1:]):
			switch strings.ToLower(fields[1]) {
			case "slack", "pv", "pq":
				return true
			}
		}
	}
	return false
}

// isDigitsOnly 判断字符串是否全部由数字组成(空串返回 false)。
func isDigitsOnly(s string) bool {
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

// runPowerflow 执行潮流网表批处理:解析 B/BR 网表 → 牛顿-拉夫逊求解 → 按 cfg.format 输出。
func runPowerflow(w io.Writer, text string, cfg config) error {
	in, err := powerflow.ParseNetlist(text)
	if err != nil {
		return fmt.Errorf("潮流网表解析失败: %w", err)
	}
	res, err := powerflow.Solve(*in, nil)
	if err != nil {
		return fmt.Errorf("潮流求解失败: %w", err)
	}
	if !cfg.quiet {
		fmt.Fprintf(os.Stderr, "潮流收敛: %v, %d 轮迭代\n", res.Converged, res.Iterations)
	}
	switch cfg.format {
	case "html":
		return writePowerflowHTML(w, in, res)
	case "table":
		return writePowerflowTable(w, in, res)
	default: // csv / tsv
		sep := ","
		if cfg.format == "tsv" {
			sep = "\t"
		}
		return writePowerflowCSV(w, in, res, sep)
	}
}

// pfBusType 母线 ID → 类型名(解析输入)。
func pfBusType(in *powerflow.Input) map[int]string {
	m := make(map[int]string, len(in.Buses))
	for _, b := range in.Buses {
		switch b.Type {
		case powerflow.BusSlack:
			m[b.ID] = "slack"
		case powerflow.BusPV:
			m[b.ID] = "pv"
		default:
			m[b.ID] = "pq"
		}
	}
	return m
}

// sortedBusIDs 母线电压表按 ID 升序。
func sortedBusIDs(v map[int]complex128) []int {
	ids := make([]int, 0, len(v))
	for id := range v {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

// sortedBranchKeys 支路潮流表按键(如 "1->2")字典序。
func sortedBranchKeys(f map[string]powerflow.BranchFlow) []string {
	keys := make([]string, 0, len(f))
	for k := range f {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// pfNum 以最短十进制表示格式化浮点数(CSV/TSV 用)。
func pfNum(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// pfFmt 以 5 位小数格式化浮点数(表格/HTML 用)。
func pfFmt(v float64) string {
	return strconv.FormatFloat(v, 'f', 5, 64)
}

// writePowerflowCSV 输出母线电压/发电机注入/支路潮流/汇总四块,各块带 # 注释头,块间空行分隔。
func writePowerflowCSV(w io.Writer, in *powerflow.Input, res *powerflow.Result, sep string) error {
	types := pfBusType(in)
	ids := sortedBusIDs(res.BusV)
	if _, err := fmt.Fprintf(w, "# bus_voltage: id,type,mag,phase_deg\n"); err != nil {
		return err
	}
	for _, id := range ids {
		mag, phase := pfMagPhase(res.BusV[id])
		line := strings.Join([]string{
			strconv.Itoa(id), types[id],
			pfNum(mag), pfNum(phase),
		}, sep)
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "# gen_power: id,p,q\n"); err != nil {
		return err
	}
	for _, id := range ids {
		s, ok := res.GenPower[id]
		if !ok {
			continue // PQ 母线无注入输出
		}
		line := strings.Join([]string{strconv.Itoa(id), pfNum(real(s)), pfNum(imag(s))}, sep)
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "# branch_flow: branch,p_from,q_from,p_to,q_to\n"); err != nil {
		return err
	}
	for _, k := range sortedBranchKeys(res.BranchFlow) {
		bf := res.BranchFlow[k]
		line := strings.Join([]string{
			k,
			pfNum(bf.PFrom), pfNum(bf.QFrom),
			pfNum(bf.PTo), pfNum(bf.QTo),
		}, sep)
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "# summary: converged,iterations,p_loss,q_loss\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, strings.Join([]string{
		strconv.FormatBool(res.Converged),
		strconv.Itoa(res.Iterations),
		pfNum(real(res.TotalLoss)), pfNum(imag(res.TotalLoss)),
	}, sep)); err != nil {
		return err
	}
	return nil
}

// writePowerflowTable 以 ASCII 表格输出母线电压、发电机注入与支路潮流。
func writePowerflowTable(w io.Writer, in *powerflow.Input, res *powerflow.Result) error {
	types := pfBusType(in)
	ids := sortedBusIDs(res.BusV)

	rows := make([]tableRow, 0, len(ids))
	for _, id := range ids {
		mag, phase := pfMagPhase(res.BusV[id])
		rows = append(rows, tableRow{
			time: strconv.Itoa(id),
			vals: []string{
				types[id],
				pfFmt(mag), pfFmt(phase),
			},
		})
	}
	if _, err := fmt.Fprintln(w, "=== 母线电压(标幺)==="); err != nil {
		return err
	}
	writeTable(w, []string{"母线", "类型", "|V|", "θ (deg)"}, rows)

	genRows := make([]tableRow, 0, len(res.GenPower))
	for _, id := range ids {
		s, ok := res.GenPower[id]
		if !ok {
			continue
		}
		genRows = append(genRows, tableRow{
			time: strconv.Itoa(id),
			vals: []string{pfFmt(real(s)), pfFmt(imag(s))},
		})
	}
	if _, err := fmt.Fprintln(w, "\n=== 发电机注入(标幺)==="); err != nil {
		return err
	}
	writeTable(w, []string{"母线", "P", "Q"}, genRows)

	brRows := make([]tableRow, 0, len(res.BranchFlow))
	for _, k := range sortedBranchKeys(res.BranchFlow) {
		bf := res.BranchFlow[k]
		brRows = append(brRows, tableRow{
			time: k,
			vals: []string{pfFmt(bf.PFrom), pfFmt(bf.QFrom), pfFmt(bf.PTo), pfFmt(bf.QTo)},
		})
	}
	if _, err := fmt.Fprintln(w, "\n=== 支路潮流(标幺,From→To)==="); err != nil {
		return err
	}
	writeTable(w, []string{"支路", "P_From", "Q_From", "P_To", "Q_To"}, brRows)

	_, err := fmt.Fprintf(w, "\n网损: P=%s, Q=%s (标幺); 收敛: %v, %d 轮迭代\n",
		pfFmt(real(res.TotalLoss)), pfFmt(imag(res.TotalLoss)), res.Converged, res.Iterations)
	return err
}

// writePowerflowHTML 输出自包含 HTML 结果页(表格)。
func writePowerflowHTML(w io.Writer, in *powerflow.Input, res *powerflow.Result) error {
	types := pfBusType(in)
	ids := sortedBusIDs(res.BusV)

	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"zh\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<title>circuit 潮流计算结果</title>\n<style>\n")
	b.WriteString("body{font-family:sans-serif;margin:2em;color:#222}\n")
	b.WriteString("table{border-collapse:collapse;margin:1em 0}\n")
	b.WriteString("th,td{border:1px solid #ccc;padding:4px 12px;text-align:right}\n")
	b.WriteString("th{background:#4A90D9;color:#fff}\ntd:first-child,th:first-child{text-align:left}\n")
	b.WriteString("h2{color:#4A90D9}\n</style>\n</head>\n<body>\n")
	fmt.Fprintf(&b, "<h1>circuit 潮流计算结果</h1>\n<p>收敛: %v · 迭代: %d 轮 · 网损: P=%s, Q=%s(标幺)</p>\n",
		res.Converged, res.Iterations, pfFmt(real(res.TotalLoss)), pfFmt(imag(res.TotalLoss)))

	b.WriteString("<h2>母线电压</h2>\n<table>\n<tr><th>母线</th><th>类型</th><th>|V| (pu)</th><th>θ (deg)</th></tr>\n")
	for _, id := range ids {
		mag, phase := pfMagPhase(res.BusV[id])
		fmt.Fprintf(&b, "<tr><td>%d</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			id, html.EscapeString(types[id]), pfFmt(mag), pfFmt(phase))
	}
	b.WriteString("</table>\n")

	b.WriteString("<h2>发电机注入</h2>\n<table>\n<tr><th>母线</th><th>P (pu)</th><th>Q (pu)</th></tr>\n")
	for _, id := range ids {
		s, ok := res.GenPower[id]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "<tr><td>%d</td><td>%s</td><td>%s</td></tr>\n",
			id, pfFmt(real(s)), pfFmt(imag(s)))
	}
	b.WriteString("</table>\n")

	b.WriteString("<h2>支路潮流</h2>\n<table>\n<tr><th>支路</th><th>P_From</th><th>Q_From</th><th>P_To</th><th>Q_To</th></tr>\n")
	for _, k := range sortedBranchKeys(res.BranchFlow) {
		bf := res.BranchFlow[k]
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			html.EscapeString(k), pfFmt(bf.PFrom), pfFmt(bf.QFrom), pfFmt(bf.PTo), pfFmt(bf.QTo))
	}
	b.WriteString("</table>\n</body>\n</html>\n")
	_, err := io.WriteString(w, b.String())
	return err
}
