package mcp

import (
	"circuit/webout"
	"fmt"
	"sort"
)

// ---------- E 组 · 结果获取与导出 ----------

// handleGetResults circuit_get_results：获取任务结果。
// 参数: sessionId(必填), jobId?(可选), format?(json|csv, 默认 json),
//
//	maxPoints?(可选, 抽稀点数, 默认 10000, 0=全部), probes[](可选, 列名过滤)
func (h *Handler) handleGetResults(args map[string]any) (any, error) {
	sess, err := h.store.Get(argStr(args, "sessionId"))
	if err != nil {
		return nil, err
	}
	job := h.lookupJob(sess, args)
	if job == nil {
		return nil, fmt.Errorf("会话 %s 没有匹配的任务（jobId 或最近任务）", sess.ID)
	}
	maxPoints := 10000
	if v, ok := argInt(args, "maxPoints"); ok {
		if v < 0 {
			return nil, fmt.Errorf("maxPoints 不能为负")
		}
		maxPoints = v
	}
	probes := argStrSlice(args, "probes")

	switch argStr(args, "format") {
	case "csv":
		return map[string]any{
			"jobId":  job.ID,
			"format": "csv",
			"csv":    job.ToCSV(maxPoints),
		}, nil
	case "", "json":
		payload := job.Result(maxPoints, probes)
		if len(probes) > 0 && len(payload.Series) == 0 {
			return nil, fmt.Errorf("探针列均不存在（可用结果 probes 字段查看列名）")
		}
		payload.JobID = job.ID
		return payload, nil
	default:
		return nil, fmt.Errorf("format 仅支持 json/csv")
	}
}

// handleExportPlot circuit_export_plot：导出自包含 HTML 波形页。
// 参数: sessionId(必填), jobId?(可选), title?(可选)
func (h *Handler) handleExportPlot(args map[string]any) (any, error) {
	sess, err := h.store.Get(argStr(args, "sessionId"))
	if err != nil {
		return nil, err
	}
	job := h.lookupJob(sess, args)
	if job == nil {
		return nil, fmt.Errorf("会话 %s 没有匹配的任务（jobId 或最近任务）", sess.ID)
	}
	title := argStr(args, "title")
	if title == "" {
		title = fmt.Sprintf("circuit 仿真结果 · %s · %s", sess.ID, job.Kind)
	}

	job.mu.Lock()
	times := append([]float64(nil), job.times...)
	series := make(map[string][]float64, len(job.series))
	for name, s := range job.series {
		series[name] = append([]float64(nil), s...)
	}
	probes := append([]ProbeSummary(nil), job.Probes...)
	steps := job.Steps
	finalTime := job.FinalTime
	job.mu.Unlock()

	if len(times) == 0 {
		return nil, fmt.Errorf("任务 %s 无采样数据", job.ID)
	}
	// 抽稀到 10000 点
	times, series = decimate(times, series, 10000)

	headers := make([]string, 0, len(probes)+1)
	headers = append(headers, "time")
	units := make([]string, 0, len(probes))
	kinds := make([]string, 0, len(probes))
	rows := make([][]float64, 0, len(times))
	names := make([]string, 0, len(probes))
	for _, p := range probes {
		if _, ok := series[p.Name]; ok {
			names = append(names, p.Name)
		}
	}
	// 列顺序：电压探针在前、电流探针在后（与 HTML 模板分组一致），同组按名称排序
	probeKind := map[string]string{}
	probeUnit := map[string]string{}
	for _, p := range probes {
		probeKind[p.Name] = p.Kind
		probeUnit[p.Name] = p.Unit
	}
	sort.SliceStable(names, func(i, j int) bool {
		ki := probeKind[names[i]]
		kj := probeKind[names[j]]
		if ki != kj {
			return ki == "node"
		}
		return names[i] < names[j]
	})
	for _, name := range names {
		headers = append(headers, name)
		units = append(units, probeUnit[name])
		kinds = append(kinds, probeKind[name])
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("任务 %s 无探针数据", job.ID)
	}
	for i := range times {
		row := make([]float64, 0, len(names)+1)
		row = append(row, times[i])
		for _, name := range names {
			row = append(row, series[name][i])
		}
		rows = append(rows, row)
	}

	meta := webout.Meta{
		Title:      title,
		Elements:   len(sess.Con.Nodelist),
		Nodes:      len(sess.Con.CompactNodeID),
		Steps:      int(steps),
		TargetTime: 0,
		FinalTime:  finalTime,
	}
	var html string
	buf := &stringWriter{}
	if err := webout.WriteHTML(buf, headers, rows, units, kinds, meta); err != nil {
		return nil, err
	}
	html = buf.String()
	return map[string]any{
		"jobId":  job.ID,
		"title":  title,
		"format": "html",
		"html":   html,
		"bytes":  len(html),
	}, nil
}

// stringWriter 简易字符串写入器。
type stringWriter struct {
	b []byte
}

// Write 实现 io.Writer:将数据追加到缓冲区。
func (w *stringWriter) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}

// String 返回已写入的字符串。
func (w *stringWriter) String() string {
	return string(w.b)
}
