package main

import (
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"

	"circuit/mna"
)

// plotColors 定义绘图曲线的 8 种颜色，按优先级分配给不同节点。
var plotColors = []color.Color{
	color.RGBA{255, 0, 0, 255},
	color.RGBA{0, 0, 255, 255},
	color.RGBA{0, 255, 255, 255},
	color.RGBA{255, 0, 255, 255},
	color.RGBA{0, 128, 0, 255},
	color.RGBA{255, 200, 0, 255},
	color.RGBA{139, 0, 0, 255},
	color.RGBA{165, 42, 42, 255},
}

// cmdPlot 执行 plot 命令：使用 gonum/plot 生成指定节点的 PNG 电压曲线图。
// 支持自适应图像尺寸和单/多节点曲线。
func (m *tuiModel) cmdPlot(args []string) {
	if len(args) < 1 {
		m.output = append(m.output, "  用法: plot <节点|all> [文件名]")
		return
	}
	nodeStr := args[0]

	filename := "plot.png"
	if len(args) >= 2 {
		filename = args[1]
		if !strings.HasSuffix(filename, ".png") {
			filename += ".png"
		}
	}

	// ---- Parse queried nodes ----
	var qn []int
	if nodeStr == "all" {
		for raw := range m.con.CompactNodeID {
			qn = append(qn, int(raw))
		}
		sort.Ints(qn)
	} else {
		for _, p := range strings.Split(nodeStr, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			n, e := strconv.Atoi(p)
			if e != nil {
				continue
			}
			if _, ok := m.con.CompactNodeID[mna.NodeID(n)]; ok {
				qn = append(qn, n)
			}
		}
	}
	if len(qn) == 0 {
		m.output = append(m.output, "  无有效节点")
		return
	}

	// ---- Get historical data ----
	m.mu.RLock()
	total := m.bufCfg.TotalRows()
	rows := m.bufCfg.LastRows(total)
	m.mu.RUnlock()

	filtered := rows
	if len(filtered) < 2 {
		m.output = append(m.output, "  数据不足（至少需要2个数据点）")
		return
	}

	// ---- Adaptive image size based on actual data points ----
	imgW := 800
	dataPts := len(filtered)
	if dataPts > 400 {
		imgW = dataPts * 3 / 2
		if imgW > 4096 {
			imgW = 4096
		}
	}
	// Height: 3:5 ratio to width, min 500, max 4096
	imgH := imgW * 3 / 5
	if imgH < 500 {
		imgH = 500
	}

	tMin := filtered[0][0]
	tMax := filtered[len(filtered)-1][0]

	// ---- Build title label ----
	nodeLabel := nodeStr
	if len(qn) > 3 && nodeStr == "all" {
		nodeLabel = fmt.Sprintf("%d nodes", len(qn))
	}
	titleStr := fmt.Sprintf("Voltage: %s  [%.3es ~ %.3es]", nodeLabel, tMin, tMax)

	// ---- Create gonum/plot ----
	p := plot.New()
	p.Title.Text = titleStr
	p.Title.TextStyle.Font.Size = vg.Points(14)
	p.X.Label.Text = "Time (s)"
	p.Y.Label.Text = "Voltage (V)"

	// Add grid
	p.Add(plotter.NewGrid())

	// ---- Plot curves ----
	for ci, n := range qn {
		idx, ok := m.con.CompactNodeID[mna.NodeID(n)]
		if !ok {
			continue
		}
		pts := make(plotter.XYs, 0, len(filtered))
		for _, r := range filtered {
			if idx+1 < len(r) {
				pts = append(pts, plotter.XY{X: r[0], Y: r[idx+1]})
			}
		}
		if len(pts) < 2 {
			continue
		}
		line, err := plotter.NewLine(pts)
		if err != nil {
			continue
		}
		line.Color = plotColors[ci%len(plotColors)]
		p.Add(line)
		if len(qn) > 1 {
			p.Legend.Add(fmt.Sprintf("node_%d", n), line)
		}
	}

	// Legend position
	if len(qn) > 1 {
		p.Legend.Top = true
		p.Legend.Left = true
	}

	// ---- Save PNG ----
	dpi := 96.0
	w := vg.Length(float64(imgW)) * vg.Inch / vg.Length(dpi)
	h := vg.Length(float64(imgH)) * vg.Inch / vg.Length(dpi)
	if err := p.Save(w, h, filename); err != nil {
		m.output = append(m.output, fmt.Sprintf("  保存失败: %v", err))
		return
	}

	m.output = append(m.output, fmt.Sprintf("  ✅ 已保存: %s (%dx%d, %d曲线, %d点, %.3e~%.3es)",
		filename, imgW, imgH, len(qn), len(filtered), tMin, tMax))
}
