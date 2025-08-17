package mna

import (
	"circuit/graph"
	"fmt"
	"net/http"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

// Debug 调试库
type Debug struct {
	IsDebug bool // 设置调试
	Line    *charts.Line
	Graph   *charts.Graph
	XAxis   *[]float64
	Items   []*[]opts.LineData
	Series  []charts.SingleSeries
}

// NewDebug 创建调试信息
func NewDebug(graph *graph.Graph) (debug *Debug) {
	debug = &Debug{
		XAxis: new([]float64),
		Graph: charts.NewGraph(),
		Line:  charts.NewLine(),
	}
	debug.Graph.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme: types.ThemeWesteros,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    "电路节点信息",
			Subtitle: "电路连接节点网络图",
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Scale: opts.Bool(true),
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Scale: opts.Bool(true),
		}),
	)
	debug.Line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme: types.ThemeWesteros,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    "电路求解状态",
			Subtitle: "电路节点电压随时间变化曲线",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			SplitNumber: 20,
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Scale: opts.Bool(true),
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:       "inside",
			Start:      50,
			End:        100,
			XAxisIndex: []int{0},
		}),
		charts.WithAnimation(true),
	)
	debug.Graph.SetSeriesOptions(
		charts.WithEmphasisOpts(opts.Emphasis{
			Label: &opts.Label{
				Show:     opts.Bool(true),
				Color:    "black",
				Position: "left",
			},
		}),
		charts.WithLineStyleOpts(opts.LineStyle{
			Curveness: 0.3,
		}),
	)
	// 初始化电路节点
	categories := []*opts.GraphCategory{
		{Name: "元件", ItemStyle: &opts.ItemStyle{Color: "#c71979b7"}},
		{Name: "节点", ItemStyle: &opts.ItemStyle{Color: "#1987c7b7"}},
	}
	graphLink := make([]opts.GraphLink, 0)
	graphNodes := make([]opts.GraphNode, len(graph.ElementList))
	graphNodeList := make([]opts.GraphNode, len(graph.NodeList)+1)
	for i, v := range graph.ElementList {
		graphNodes[int(i)] = opts.GraphNode{
			Name:     fmt.Sprintf("%s(%d)", v.Type(), v.ID),
			Category: 0,
			Tooltip:  &opts.Tooltip{Show: opts.Bool(true)},
		}
	}
	for i := range graph.NodeList {
		graphNodeList[int(i)] = opts.GraphNode{
			Name:     fmt.Sprintf("Node(%d)", i+1),
			Category: 1,
			Tooltip:  &opts.Tooltip{Show: opts.Bool(true)},
		}
	}
	graphNodeList[len(graphNodeList)-1] = opts.GraphNode{
		Name:      "Gnd",
		Category:  1,
		ItemStyle: &opts.ItemStyle{Color: "#000000de"},
		Tooltip:   &opts.Tooltip{Show: opts.Bool(true)},
	}
	for i, v := range graph.ElementList {
		for _, n := range v.Nodes {
			if n != -1 {
				graphLink = append(graphLink, opts.GraphLink{
					Source: graphNodes[int(i)].Name,
					Target: graphNodeList[int(n)].Name,
				})
			} else {
				graphLink = append(graphLink, opts.GraphLink{
					Source: graphNodes[int(i)].Name,
					Target: "Gnd",
				})
			}
		}
	}
	debug.Graph.AddSeries("电路列表", append(graphNodes, graphNodeList...), graphLink,
		charts.WithGraphChartOpts(opts.GraphChart{
			Categories:         categories,
			Roam:               opts.Bool(true),
			Force:              &opts.GraphForce{Repulsion: 80},
			EdgeLabel:          &opts.EdgeLabel{Show: opts.Bool(true)},
			FocusNodeAdjacency: opts.Bool(true),
		}))
	// 初始化节点信息
	n := graph.NumNodes + graph.NumVoltageSources
	debug.Items = make([]*[]opts.LineData, n)
	debug.Series = make([]charts.SingleSeries, n)
	for i := range n {
		debug.Items[i] = new([]opts.LineData)
		debug.Series[i].Data = debug.Items[i]
		debug.Series[i].Name = fmt.Sprintf("%d", i)
		debug.Series[i].Type = types.ChartLine
		debug.Series[i].InitSeriesDefaultOpts(debug.Line.BaseConfiguration)
	}
	debug.Line.SetXAxis(debug.XAxis)
	return debug
}

// Update 记录数据
func (debug *Debug) Update(mna *MNA) {
	data := mna.OrigX.RawVector().Data
	for i, f := range data {
		*debug.Items[i] = append(*debug.Items[i], opts.LineData{Value: f})
	}
	*debug.XAxis = append(*debug.XAxis, mna.Time)
}

// Handler 发布到网页面
func (debug *Debug) Handler(w http.ResponseWriter, _ *http.Request) {
	debug.Line.MultiSeries = debug.Series
	page := components.NewPage()
	page.AddCharts(
		debug.Graph,
		debug.Line,
	)
	page.Render(w)
}
