package debug

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

// Charts 曲线绘制
type Charts struct {
	Record
}

// Render 格式化
func (c *Charts) Render(w io.Writer) error {
	// 初始化界面
	graph := charts.NewGraph()
	graph.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme: types.ThemeWesteros,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    "电路节点信息",
			Subtitle: "电路连接节点网络图",
		}),
		charts.WithLegendOpts(opts.Legend{
			Type:   "scroll",
			Orient: "vertical",
			Right:  "10",
			Top:    "20",
			Bottom: "20",
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Scale: opts.Bool(true),
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Scale: opts.Bool(true),
		}),
	)
	graph.SetSeriesOptions(
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
	lineV := charts.NewLine()
	lineV.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme: types.ThemeWesteros,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    "电压曲线",
			Subtitle: "电路节点电压随时间变化曲线",
		}),
		charts.WithLegendOpts(opts.Legend{
			Type:   "scroll",
			Orient: "vertical",
			Right:  "10",
			Top:    "20",
			Bottom: "20",
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
	lineA := charts.NewLine()
	lineA.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title:    "电流曲线",
			Subtitle: "电路节点电流随时间变化曲线",
		}),
		charts.WithInitializationOpts(opts.Initialization{
			Theme: types.ThemeWesteros,
		}),
		charts.WithLegendOpts(opts.Legend{
			Type:   "scroll",
			Orient: "vertical",
			Right:  "10",
			Top:    "20",
			Bottom: "20",
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
	lineI := charts.NewLine()
	lineI.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme: types.ThemeWesteros,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    "激励曲线",
			Subtitle: "电路节点激励随时间变化曲线",
		}),
		charts.WithLegendOpts(opts.Legend{
			Type:   "scroll",
			Orient: "vertical",
			Right:  "10",
			Top:    "20",
			Bottom: "20",
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
	// 处理数据
	{
		// 初始化电路节点
		graphLink := make([]opts.GraphLink, 0)
		graphNodes := make([]opts.GraphNode, len(c.Elements))
		graphNodeList := make([]opts.GraphNode, len(c.Nodes))
		for i, n := range c.Elements {
			graphNodes[i] = opts.GraphNode{
				Name:     n,
				Category: 0,
				Tooltip:  &opts.Tooltip{Show: opts.Bool(true)},
			}
		}
		graphNodes[0].ItemStyle = &opts.ItemStyle{Color: "#000000de"}
		for i, n := range c.Nodes {
			if i == 0 {
				// 处理地线
				for _, y := range n {
					graphLink = append(graphLink, opts.GraphLink{
						Source: graphNodes[y[0]].Name,
						Target: graphNodes[0].Name,
						Value:  float32(y[1]),
					})
				}
			} else {
				// 处理连接信息
				graphNodeList[i] = opts.GraphNode{
					Name:     fmt.Sprintf("Node(%d)", i),
					Category: 1,
					Tooltip:  &opts.Tooltip{Show: opts.Bool(true)},
				}
				for _, y := range n {
					graphLink = append(graphLink, opts.GraphLink{
						Source: graphNodes[y[0]].Name,
						Target: graphNodeList[i].Name,
						Value:  float32(y[1]),
					})
				}
			}
		}
		graph.AddSeries("电路列表", append(graphNodes, graphNodeList[1:]...), graphLink,
			charts.WithGraphChartOpts(opts.GraphChart{
				Categories: []*opts.GraphCategory{
					{Name: "元件", ItemStyle: &opts.ItemStyle{Color: "#c71979b7"}},
					{Name: "节点", ItemStyle: &opts.ItemStyle{Color: "#1987c7b7"}},
				},
				Roam:               opts.Bool(true),
				Force:              &opts.GraphForce{Repulsion: 80},
				EdgeLabel:          &opts.EdgeLabel{Show: opts.Bool(true)},
				FocusNodeAdjacency: opts.Bool(true),
			}))
		// 电压信息
		{
			lineV.SetXAxis(c.Time)
			itemsV := make([][]opts.LineData, 0)
			seriesV := make([]charts.SingleSeries, 0)
			for i := range c.Voltage[0] {
				itemsV = append(itemsV, make([]opts.LineData, len(c.Time)))
				seriesV = append(seriesV, charts.SingleSeries{
					Name: fmt.Sprintf("Node(%d)", i+1),
					Data: itemsV[i],
					Type: types.ChartLine,
				})
				seriesV[i].InitSeriesDefaultOpts(lineV.BaseConfiguration)
			}
			for i, v := range c.Voltage {
				for x, t := range v {
					itemsV[x][i].Value = t
				}
			}
			lineV.MultiSeries = seriesV
		}
		// 电流信息
		{
			lineA.SetXAxis(c.Time)
			itemsA := make([][]opts.LineData, 0)
			seriesA := make([]charts.SingleSeries, 0)
			for i := range c.Current[0] {
				itemsA = append(itemsA, make([]opts.LineData, len(c.Time)))
				seriesA = append(seriesA, charts.SingleSeries{
					Name: c.CurrentStr[i],
					Data: itemsA[i],
					Type: types.ChartLine,
				})
				seriesA[i].InitSeriesDefaultOpts(lineA.BaseConfiguration)
			}
			for i, v := range c.Current {
				for x, t := range v {
					itemsA[x][i].Value = t
				}
			}
			lineA.MultiSeries = seriesA
		}
		// 激励信息
		{
			lineI.SetXAxis(c.Time)
			itemsI := make([][]opts.LineData, 0)
			seriesI := make([]charts.SingleSeries, 0)
			for i := range c.Incentive[0] {
				itemsI = append(itemsI, make([]opts.LineData, len(c.Time)))
				seriesI = append(seriesI, charts.SingleSeries{
					Name: fmt.Sprintf("%d", i),
					Data: itemsI[i],
					Type: types.ChartLine,
				})
				seriesI[i].InitSeriesDefaultOpts(lineI.BaseConfiguration)
			}
			for i, v := range c.Incentive {
				for x, t := range v {
					itemsI[x][i].Value = t
				}
			}
			lineI.MultiSeries = seriesI
		}
	}
	// 构建界面
	page := components.NewPage()
	page.AddCharts(
		graph,
		lineV,
		lineA,
		lineI,
	)
	return page.Render(w)
}

// Handler 发布到网页面
func (c *Charts) Handler(w http.ResponseWriter, _ *http.Request) {
	c.Render(w)
}

func (c *Charts) Error(err error) { log.Println(err) }
