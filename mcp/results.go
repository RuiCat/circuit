package mcp

import (
	"strings"
)

// decimate 均匀抽稀到 maxPoints 点（<=0 表示不抽稀）。
func decimate(times []float64, series map[string][]float64, maxPoints int) ([]float64, map[string][]float64) {
	n := len(times)
	if maxPoints <= 0 || n <= maxPoints {
		return times, series
	}
	step := float64(n) / float64(maxPoints)
	idxs := make([]int, 0, maxPoints)
	for i := 0; i < maxPoints; i++ {
		idxs = append(idxs, int(float64(i)*step))
	}
	nt := make([]float64, 0, len(idxs))
	for _, i := range idxs {
		nt = append(nt, times[i])
	}
	ns := make(map[string][]float64, len(series))
	for name, s := range series {
		col := make([]float64, 0, len(idxs))
		for _, i := range idxs {
			col = append(col, s[i])
		}
		ns[name] = col
	}
	return nt, ns
}

// ResultPayload 仿真结果载荷。
type ResultPayload struct {
	JobID     string               `json:"jobId"`
	Kind      string               `json:"kind"`
	State     JobState             `json:"state"`
	Steps     int64                `json:"steps"`
	FinalTime float64              `json:"finalTime"`
	Time      []float64            `json:"time"`             // 抽稀后的时间轴
	Series    map[string][]float64 `json:"series"`           // 抽稀后的各探针序列
	Probes    []ProbeSummary       `json:"probes"`           // 探针摘要（min/max/last）
	FinalV    map[string]float64   `json:"finalV,omitempty"` // 终态节点电压快照
	FinalI    map[string]float64   `json:"finalI,omitempty"` // 终态元件电流快照
	Error     string               `json:"error,omitempty"`
}

// Result 获取任务结果（带锁快照）。
func (j *Job) Result(maxPoints int, probeFilter []string) ResultPayload {
	j.mu.Lock()
	defer j.mu.Unlock()
	series := make(map[string][]float64, len(probeFilter))
	if len(probeFilter) == 0 {
		for name, s := range j.series {
			series[name] = append([]float64(nil), s...)
		}
	} else {
		for _, name := range probeFilter {
			if s, ok := j.series[name]; ok {
				series[name] = append([]float64(nil), s...)
			}
		}
	}
	times := append([]float64(nil), j.times...)
	times, series = decimate(times, series, maxPoints)

	finalV := make(map[string]float64, len(j.finalV))
	for k, v := range j.finalV {
		finalV[k] = v
	}
	finalI := make(map[string]float64, len(j.finalI))
	for k, v := range j.finalI {
		finalI[k] = v
	}
	return ResultPayload{
		JobID:     j.ID,
		Kind:      j.Kind,
		State:     j.State,
		Steps:     j.Steps,
		FinalTime: j.FinalTime,
		Time:      times,
		Series:    series,
		Probes:    append([]ProbeSummary(nil), j.Probes...),
		FinalV:    finalV,
		FinalI:    finalI,
		Error:     j.Error,
	}
}

// ToCSV 将任务结果编码为 CSV 文本。
func (j *Job) ToCSV(maxPoints int) string {
	j.mu.Lock()
	times := append([]float64(nil), j.times...)
	series := make(map[string][]float64, len(j.series))
	for name, s := range j.series {
		series[name] = append([]float64(nil), s...)
	}
	probes := append([]ProbeSummary(nil), j.Probes...)
	j.mu.Unlock()

	times, series = decimate(times, series, maxPoints)
	var sb strings.Builder
	sb.WriteString("time")
	for _, p := range probes {
		if _, ok := series[p.Name]; ok {
			sb.WriteString(",")
			sb.WriteString(p.Name)
		}
	}
	sb.WriteString("\n")
	for i := range times {
		sb.WriteString(formatFloat(times[i]))
		for _, p := range probes {
			col, ok := series[p.Name]
			if !ok || i >= len(col) {
				continue
			}
			sb.WriteString(",")
			sb.WriteString(formatFloat(col[i]))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
