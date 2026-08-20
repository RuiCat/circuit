package main

import (
	"fmt"
	"image/color"
	"path/filepath"
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
		// 只取 basename，防止路径遍历写任意位置。
		base := filepath.Base(args[1])
		if base != "." && base != string(filepath.Separator) && base != "" {
			filename = base
		}
		if !strings.HasSuffix(filename, ".png") {
			filename += ".png"
		}
	}

	// ---- 解析查询节点 ----
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

	// ---- 获取历史数据 ----
	m.mu.RLock()
	total := m.bufCfg.TotalRows()
	rows := m.bufCfg.LastRows(total)
	m.mu.RUnlock()

	filtered := rows
	if len(filtered) < 2 {
		m.output = append(m.output, "  数据不足（至少需要2个数据点）")
		return
	}

	// ---- 按实际数据点数自适应图像尺寸 ----
	imgW := 800
	dataPts := len(filtered)
	if dataPts > 400 {
		imgW = dataPts * 3 / 2
		if imgW > 4096 {
			imgW = 4096
		}
	}
	// 高度:宽度的 3:5,最小 500,最大 4096
	imgH := imgW * 3 / 5
	if imgH < 500 {
		imgH = 500
	}

	tMin := filtered[0][0]
	tMax := filtered[len(filtered)-1][0]

	// ---- 构建标题 ----
	nodeLabel := nodeStr
	if len(qn) > 3 && nodeStr == "all" {
		nodeLabel = fmt.Sprintf("%d nodes", len(qn))
	}
	titleStr := fmt.Sprintf("Voltage: %s  [%.3es ~ %.3es]", nodeLabel, tMin, tMax)

	// ---- 创建 gonum/plot 绘图对象 ----
	p := plot.New()
	p.Title.Text = titleStr
	p.Title.TextStyle.Font.Size = vg.Points(14)
	p.X.Label.Text = "Time (s)"
	p.Y.Label.Text = "Voltage (V)"

	// 添加网格
	p.Add(plotter.NewGrid())

	// ---- 绘制曲线 ----
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

	// 图例位置
	if len(qn) > 1 {
		p.Legend.Top = true
		p.Legend.Left = true
	}

	// ---- 保存 PNG ----
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
