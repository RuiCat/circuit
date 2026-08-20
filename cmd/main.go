// Command circuit 是电路仿真命令行工具，支持批处理仿真、交互式 TUI 连续仿真与 MCP 服务器三种模式。
// 基于 bubbletea 的终端界面提供实时电压监控、事件设置和曲线绘制功能。
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"circuit/element"
	_ "circuit/element/register"
	etime "circuit/element/time"
	"circuit/load"
	"circuit/mcp"
	"circuit/mna"
	"circuit/webout"

	"github.com/mark3labs/mcp-go/server"
)

// errWriter 包装 io.Writer，记录首次写入错误。
// 后续写入自动丢弃，避免静默数据丢失。
type errWriter struct {
	w   io.Writer
	err error
}

// Write 实现 io.Writer:记录首次错误,后续写入自动丢弃。
func (ew *errWriter) Write(p []byte) (int, error) {
	if ew.err != nil {
		return 0, ew.err
	}
	var n int
	n, ew.err = ew.w.Write(p)
	return n, ew.err
}

// config 存储命令行解析后的仿真配置参数。
type config struct {
	mode        string
	targetTime  float64
	outputPath  string
	initialStep float64
	minStep     float64
	maxStep     float64
	absTol      float64
	relTol      float64
	maxIter     int
	maxSteps    int
	nodeFilter  []int
	parallel    int
	format      string
	quiet       bool
	showHelp    bool
	netlistPath string
}

// main 程序入口：解析参数、加载网表、运行仿真、输出结果。
// 支持批处理、交互式（interactive）与 MCP 服务器（mcpserver）三种模式。
func main() {
	// MCP 服务器子命令拥有独立的 flag 集，须在全局 flag 解析前拦截
	if len(os.Args) > 1 && os.Args[1] == "mcpserver" {
		os.Exit(runMCPServer(os.Args[2:]))
	}

	cfg := parseFlags()

	if cfg.parallel < 0 {
		fmt.Fprintf(os.Stderr, "警告: --parallel 不能为负值 (%d)，已重置为 0\n", cfg.parallel)
		cfg.parallel = 0
	}
	if cfg.parallel == 1 {
		cfg.parallel = 0
	}

	if cfg.showHelp {
		printHelp()
		os.Exit(0)
	}

	if cfg.format != "csv" && cfg.format != "tsv" && cfg.format != "table" && cfg.format != "html" {
		fmt.Fprintf(os.Stderr, "错误: 不支持的输出格式 '%s'，可选: csv, tsv, table, html\n", cfg.format)
		os.Exit(1)
	}

	if cfg.netlistPath == "" {
		printHelp()
		os.Exit(1)
	}

	netlistFile, err := os.Open(cfg.netlistPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 无法打开文件 %s: %v\n", cfg.netlistPath, err)
		os.Exit(2)
	}
	defer func() {
		if cerr := netlistFile.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "警告: 关闭网表文件失败: %v\n", cerr)
		}
	}()

	con, err := load.LoadContext(netlistFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 网表加载失败: %v\n", err)
		os.Exit(3)
	}

	if cfg.mode == "interactive" {
		if err := runTUI(con, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(4)
		}
		return
	}

	var w io.Writer
	var outFile *os.File
	var bufWriter *bufio.Writer

	if cfg.outputPath == "" || cfg.outputPath == "stdout" {
		w = os.Stdout
	} else {
		outFile, err = os.Create(cfg.outputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: 无法创建输出文件 %s: %v\n", cfg.outputPath, err)
			os.Exit(2)
		}
		defer func() {
			if cerr := outFile.Close(); cerr != nil {
				fmt.Fprintf(os.Stderr, "警告: 关闭输出文件失败: %v\n", cerr)
			}
		}()
		bufWriter = bufio.NewWriter(outFile)
		w = bufWriter
	}

	if err := runSim(w, con, cfg); err != nil {
		if bufWriter != nil {
			bufWriter.Flush()
		}
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(4)
	}
	if bufWriter != nil {
		if ferr := bufWriter.Flush(); ferr != nil {
			fmt.Fprintf(os.Stderr, "错误: 刷新输出缓冲区失败: %v\n", ferr)
			os.Exit(4)
		}
	}
}

// parseFlags 解析命令行参数并返回 config 结构体。
func parseFlags() config {
	var cfg config

	flag.BoolVar(&cfg.showHelp, "h", false, "显示帮助")
	flag.BoolVar(&cfg.showHelp, "help", false, "显示帮助")

	flag.Float64Var(&cfg.targetTime, "t", 0.1, "仿真目标时间（秒）")
	flag.Float64Var(&cfg.targetTime, "time", 0.1, "仿真目标时间（秒）")

	flag.StringVar(&cfg.outputPath, "o", "stdout", "输出文件路径")
	flag.StringVar(&cfg.outputPath, "output", "stdout", "输出文件路径")

	flag.Float64Var(&cfg.initialStep, "step", 1e-6, "初始步长")
	flag.Float64Var(&cfg.minStep, "min-step", 1e-9, "最小步长")
	flag.Float64Var(&cfg.maxStep, "max-step", 5e-4, "最大步长")
	flag.Float64Var(&cfg.absTol, "abs-tol", 1e-6, "绝对容差")
	flag.Float64Var(&cfg.relTol, "rel-tol", 1e-4, "相对容差")

	flag.IntVar(&cfg.maxIter, "max-iter", 200, "最大非线性迭代次数")
	flag.IntVar(&cfg.maxSteps, "max-steps", 10000, "最大时间步数")
	flag.IntVar(&cfg.parallel, "parallel", 0, "并行 worker 数（0=串行模式）")

	var nodeStr string
	flag.StringVar(&nodeStr, "nodes", "", "逗号分隔的要输出的原始节点ID列表")

	flag.StringVar(&cfg.format, "format", "csv", "输出格式: csv, tsv, table, html")

	flag.BoolVar(&cfg.quiet, "quiet", false, "静默模式，不输出元信息到 stderr")

	flag.Usage = func() {
		printHelp()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) > 0 && args[0] == "interactive" {
		cfg.mode = "interactive"
		if len(args) > 1 {
			cfg.netlistPath = args[1]
		}
	} else if len(args) > 0 {
		cfg.netlistPath = args[0]
	}

	if nodeStr != "" {
		parts := strings.Split(nodeStr, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			n, err := strconv.Atoi(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "警告: 无效节点ID '%s'，已忽略\n", p)
				continue
			}
			cfg.nodeFilter = append(cfg.nodeFilter, n)
		}
	}

	return cfg
}

// runMCPServer 运行 MCP(Model Context Protocol) 服务器子命令。
// 通过 stdio 或 HTTP(SSE/streamable HTTP) 传输向 MCP 客户端提供电路仿真能力。
//
// 用法:
//
//	circuit mcpserver                          # stdio 模式（默认，供 Claude Desktop / dsh 等）
//	circuit mcpserver -transport http -addr :18080   # streamable HTTP 模式
//	circuit mcpserver -transport sse -addr :18080    # SSE 模式
//	circuit mcpserver -session-ttl 30m               # 会话空闲 30 分钟自动清理
func runMCPServer(args []string) int {
	fs := flag.NewFlagSet("mcpserver", flag.ExitOnError)
	var (
		transport  = fs.String("transport", "stdio", "传输方式: stdio / http / sse")
		addr       = fs.String("addr", ":18080", "HTTP 监听地址（http/sse 模式）")
		sessionTTL = fs.Duration("session-ttl", time.Hour, "会话空闲自动清理 TTL（0=不清理）")
		showHelp   = fs.Bool("h", false, "显示帮助")
	)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `circuit mcpserver - 电路仿真 MCP 服务器

用法:
  circuit mcpserver [选项]

选项:
  -transport string   传输方式: stdio / http / sse（默认: stdio）
  -addr string        HTTP 监听地址（http/sse 模式，默认: :18080）
  -session-ttl duration 会话空闲自动清理 TTL（默认: 1h，0=不清理）
  -h                  显示帮助

示例:
  circuit mcpserver                              # stdio（Claude Desktop 配置用）
  circuit mcpserver -transport http -addr :18080
`)
	}
	fs.Parse(args)
	if *showHelp {
		fs.Usage()
		return 0
	}

	store := mcp.NewSessionStore(*sessionTTL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go store.CleanupLoop(ctx.Done())

	mcpServer := mcp.NewMCPServer(store)
	log.Printf("circuit MCP 服务器启动 (version %s, 传输: %s)", mcp.Version, *transport)

	switch *transport {
	case "stdio":
		if err := server.ServeStdio(mcpServer); err != nil {
			log.Fatalf("stdio 服务失败: %v", err)
		}
	case "http", "streamable-http":
		httpServer := server.NewStreamableHTTPServer(mcpServer)
		go func() {
			<-ctx.Done()
			shutdownCtx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer scancel()
			_ = httpServer.Shutdown(shutdownCtx)
		}()
		log.Printf("streamable HTTP 监听 %s (POST /mcp)", *addr)
		if err := httpServer.Start(*addr); err != nil {
			log.Fatalf("HTTP 服务失败: %v", err)
		}
	case "sse":
		sseServer := server.NewSSEServer(mcpServer)
		go func() {
			<-ctx.Done()
			shutdownCtx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer scancel()
			_ = sseServer.Shutdown(shutdownCtx)
		}()
		log.Printf("SSE 监听 %s (/sse)", *addr)
		if err := sseServer.Start(*addr); err != nil {
			log.Fatalf("SSE 服务失败: %v", err)
		}
	default:
		fmt.Fprintf(os.Stderr, "错误: 不支持的传输方式 %q（可选: stdio / http / sse）\n", *transport)
		return 1
	}

	// 优雅退出
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	cancel()
	log.Printf("circuit MCP 服务器退出")
	return 0
}

// printHelp 输出命令行帮助信息到 stderr。
func printHelp() {
	fmt.Fprint(os.Stderr, `circuit - 电路仿真命令行工具
用法:
  circuit [选项] <网表文件>             批处理仿真
  circuit [选项] interactive <网表文件>  交互式连续仿真
  circuit mcpserver [选项]              启动 MCP 服务器（电路仿真 MCP，详见 "circuit mcpserver -h"）

选项:
  -t, --time float     仿真目标时间（秒）(默认: 0.1)
  -o, --output string  输出文件路径（默认: stdout）
      --step float     初始步长（默认: 1e-6）
      --min-step float 最小步长（默认: 1e-9）
      --max-step float 最大步长（默认: 5e-4）
      --abs-tol float  绝对容差（默认: 1e-6）
      --rel-tol float  相对容差（默认: 1e-4）
      --max-iter int   最大非线性迭代次数（默认: 200）
      --max-steps int  最大时间步数（默认: 10000）
      --nodes string   逗号分隔的要输出的原始节点ID列表，如 "1,2,5"（默认: 全部）
      --parallel int   并行 worker 数（0=串行模式，默认: 0）
      --format string  输出格式: csv, tsv, table, html（默认: csv）
      --quiet          静默模式，不输出元信息到 stderr
  -h, --help           显示帮助

示例:
  circuit circuit.net
  circuit -t 0.01 --nodes 1,2,5 -o result.csv circuit.net
  circuit --parallel 4 --format table circuit.net
  circuit -t 0.05 --format html -o result.html circuit.net
`)
}

// maxTableRows 限制终端表格输出的最大行数，超出后自动降级为 csv 格式。
const maxTableRows = 10000

// tableRow 表示表格输出的一行数据，包含时间和多个节点电压值。
type tableRow struct {
	time string
	vals []string
}

// htmlCol 描述 HTML 单网页输出的一个数据列（节点电压或元件电流）。
// read 从当前步的求解状态读取该列的值。
type htmlCol struct {
	header string                    // 列名，如 "node_1"、"I(V1)"、"I(L1.Ie)"
	unit   string                    // 单位："V"、"A"（气动/液压元件为流量单位）
	kind   string                    // 曲线分组："voltage" 或 "current"
	read   func(v []float64) float64 // 读取该列当前步的值
}

// unitOf 返回元件电流/流量列的单位：元件含气动引脚则为质量流量(kg/s)，
// 含液压引脚则为体积流量(m³/s)，否则为安培(A)。
// 覆盖普通元件与压力源/转换器内部的"电压源电流"（其电流实际是气/油流量）。
func unitOf(cfg *element.Config) string {
	for _, p := range cfg.Pin {
		switch p.Type {
		case element.PinPneumatic:
			return "kg/s"
		case element.PinHydraulic:
			return "m³/s"
		}
	}
	return "A"
}

// nodeUnitOf 返回某系统类型节点的"电压"列单位：气动/液压节点是压力(Pa)，其余为电压(V)。
func nodeUnitOf(pinType element.PinType) string {
	switch pinType {
	case element.PinPneumatic, element.PinHydraulic:
		return "Pa"
	}
	return "V"
}

// runSim 执行完整的批处理仿真流程，包括时间控制器设置、节点筛选、步进执行和结果输出。
func runSim(w io.Writer, con *element.Context, cfg config) error {
	ew := &errWriter{w: w}

	timeMNA, err := etime.NewTimeMNA(cfg.targetTime)
	if err != nil {
		return fmt.Errorf("创建时间控制器失败: %w", err)
	}
	con.Time = timeMNA

	// 初始化恢复条件变量并连接到时间管理器
	con.InitResumeCond()
	if tm, ok := con.Time.(*etime.TimeMNA); ok {
		tm.SetNotifier(func() {
			con.ResumeCond().Broadcast()
		})
	}

	if err := con.Time.SetTolerances(cfg.absTol, cfg.relTol); err != nil {
		return fmt.Errorf("设置容差失败: %w", err)
	}
	if err := con.Time.SetStepLimits(cfg.minStep, cfg.maxStep); err != nil {
		return fmt.Errorf("设置步长限制失败: %w", err)
	}

	elemIter := timeMNA.MaxElemIter()
	if err := con.Time.SetIterationLimits(cfg.maxIter, elemIter); err != nil {
		return fmt.Errorf("设置迭代限制失败: %w", err)
	}
	if err := con.Time.SetMaxTimeSteps(cfg.maxSteps); err != nil {
		return fmt.Errorf("设置最大步数失败: %w", err)
	}
	con.Time.SetTimeStep(cfg.initialStep)

	if cfg.parallel > 0 {
		con.ParallelOpts = &element.ParallelOptions{StampWorkers: cfg.parallel}
	}

	outputNodes := cfg.nodeFilter
	if len(outputNodes) == 0 {
		for rawID := range con.CompactNodeID {
			outputNodes = append(outputNodes, int(rawID))
		}
		sort.Ints(outputNodes)
	} else {
		validNodes := make([]int, 0, len(outputNodes))
		for _, n := range outputNodes {
			if _, ok := con.CompactNodeID[mna.NodeID(n)]; ok {
				validNodes = append(validNodes, n)
			} else {
				fmt.Fprintf(os.Stderr, "警告: 节点 %d 不在网表中，已忽略\n", n)
			}
		}
		outputNodes = validNodes
	}

	headers := buildHeaders(outputNodes)

	// 节点系统类型：通过连接元件的引脚类型推断（电气/气动/液压三套独立系统）。
	// 同一节点仅允许同域引脚连接（加载时已校验跨域混接），此处按首个引脚类型推断单位；
	// 转换器(EP/EH)两侧引脚分属不同系统但连接不同节点，不会冲突。
	nodePinType := make(map[int]element.PinType)
	for _, elem := range con.Nodelist {
		cfg := elem.Config()
		for i := range cfg.Pin {
			compactIdx := int(elem.GetNodes(i))
			if compactIdx < 0 {
				continue
			}
			if _, ok := nodePinType[compactIdx]; !ok {
				nodePinType[compactIdx] = cfg.Pin[i].Type
			}
		}
	}

	// html 模式的数据列：节点电压列 + 元件电流列（电压源电流 + Config.Current 声明索引）。
	// 电流值直接记录元件/求解器当前值，不做写入一致性过滤。
	var htmlCols []htmlCol
	for _, rawID := range outputNodes {
		rawID := rawID
		compactIdx, ok := con.CompactNodeID[mna.NodeID(rawID)]
		unit := "V"
		if pinType, ok2 := nodePinType[compactIdx]; ok2 {
			unit = nodeUnitOf(pinType)
		}
		htmlCols = append(htmlCols, htmlCol{
			header: fmt.Sprintf("node_%d", rawID),
			unit:   unit,
			kind:   "voltage",
			read: func(v []float64) float64 {
				if ok && compactIdx < len(v) {
					return v[compactIdx]
				}
				return 0
			},
		})
	}
	for _, elem := range con.Nodelist {
		cfg := elem.Config()
		inst := elem.Base().InstanceName
		if inst == "" {
			inst = cfg.GetName()
		}
		// 1) 电压源支路电流（mna 求解器直接给出）
		for vs := 0; vs < cfg.VoltageNum(); vs++ {
			vs := vs
			vsID := elem.GetVoltSource(vs)
			name := fmt.Sprintf("I(%s)", inst)
			if cfg.VoltageNum() > 1 {
				name = fmt.Sprintf("I(%s[%d])", inst, vs)
			}
			htmlCols = append(htmlCols, htmlCol{
				header: name,
				unit:   unitOf(cfg),
				kind:   "current",
				read: func(v []float64) float64 {
					return con.GetVoltageSourceCurrent(vsID)
				},
			})
		}
		// 2) 元件声明的自身电流索引（Config.Current）
		for _, idx := range cfg.Current {
			idx := idx
			if idx < 0 || idx >= cfg.ValueNum() {
				continue
			}
			// 只记录数值槽位：int/bool/string 槽（如单向阀 state、开关状态）不是电流，
			// 跳过以避免 GetFloat64 类型断言警告与错误数据。
			switch cfg.ValueInit[idx].(type) {
			case float64, nil:
			default:
				continue
			}
			name := fmt.Sprintf("I(%s)", inst)
			if len(cfg.Current) > 1 {
				suffix := cfg.ValueName[idx]
				if suffix == "" {
					suffix = fmt.Sprintf("i%d", idx)
				}
				name = fmt.Sprintf("I(%s.%s)", inst, suffix)
			}
			htmlCols = append(htmlCols, htmlCol{
				header: name,
				unit:   unitOf(cfg),
				kind:   "current",
				read: func(v []float64) float64 {
					return elem.GetFloat64(idx)
				},
			})
		}
	}

	if !cfg.quiet {
		fmt.Fprintf(os.Stderr, "加载成功: %d 个元件, %d 个节点\n",
			len(con.Nodelist), len(con.CompactNodeID))
		fmt.Fprintf(os.Stderr, "仿真开始: 目标时间 %.3e 秒\n", cfg.targetTime)
	}

	var tableRows []tableRow
	var htmlRows [][]float64
	stepCount := 0

	call := func(v []float64) {
		t := con.CurrentTime()
		row := tableRow{
			time: formatFloat(t),
			vals: make([]string, len(outputNodes)),
		}
		for i, rawID := range outputNodes {
			compactIdx, ok := con.CompactNodeID[mna.NodeID(rawID)]
			if ok && compactIdx < len(v) {
				row.vals[i] = formatFloat(v[compactIdx])
			} else {
				row.vals[i] = "0"
			}
		}

		switch cfg.format {
		case "csv":
			if ew.err != nil {
				return
			}
			if stepCount == 0 {
				fmt.Fprintln(ew, strings.Join(headers, ","))
			}
			fmt.Fprintln(ew, strings.Join(row.toCells(), ","))
		case "tsv":
			if ew.err != nil {
				return
			}
			if stepCount == 0 {
				fmt.Fprintln(ew, strings.Join(headers, "\t"))
			}
			fmt.Fprintln(ew, strings.Join(row.toCells(), "\t"))
		case "table":
			tableRows = append(tableRows, row)
			if len(tableRows) > maxTableRows && cfg.format == "table" {
				if !cfg.quiet {
					fmt.Fprintf(os.Stderr, "警告: 步数超过 %d，自动降级为 csv 格式\n", maxTableRows)
				}
				cfg.format = "csv"
				if ew.err == nil {
					fmt.Fprintln(ew, strings.Join(headers, ","))
					for _, r := range tableRows {
						if ew.err != nil {
							break
						}
						fmt.Fprintln(ew, strings.Join(r.toCells(), ","))
					}
				}
				tableRows = nil
			}
		case "html":
			if ew.err != nil {
				return
			}
			row := make([]float64, 1+len(htmlCols))
			row[0] = t
			for i, c := range htmlCols {
				row[i+1] = c.read(v)
			}
			htmlRows = append(htmlRows, row)
		}
		stepCount++
	}

	if err := etime.TransientSimulation(con, call); err != nil {
		return fmt.Errorf("仿真失败: %w", err)
	}

	if ew.err != nil {
		return fmt.Errorf("输出写入失败: %w", ew.err)
	}

	if cfg.format == "table" {
		writeTable(ew, headers, tableRows)
	}

	if cfg.format == "html" {
		htmlHeaders := make([]string, 0, 1+len(htmlCols))
		htmlHeaders = append(htmlHeaders, "time")
		units := make([]string, len(htmlCols))
		kinds := make([]string, len(htmlCols))
		for i, c := range htmlCols {
			htmlHeaders = append(htmlHeaders, c.header)
			units[i] = c.unit
			kinds[i] = c.kind
		}
		if err := webout.WriteHTML(ew, htmlHeaders, htmlRows, units, kinds, webout.Meta{
			Title:      fmt.Sprintf("circuit 仿真结果 · %s", filepath.Base(cfg.netlistPath)),
			Elements:   len(con.Nodelist),
			Nodes:      len(con.CompactNodeID),
			Steps:      stepCount,
			TargetTime: cfg.targetTime,
			FinalTime:  con.CurrentTime(),
		}); err != nil {
			return fmt.Errorf("写入 HTML 输出失败: %w", err)
		}
	}

	if !cfg.quiet {
		fmt.Fprintf(os.Stderr, "仿真完成: %d 步, 最终时间 %.3e 秒\n",
			stepCount, con.CurrentTime())
	}

	if ew.err != nil {
		return fmt.Errorf("输出写入失败: %w", ew.err)
	}

	return nil
}

// buildHeaders 根据节点 ID 列表构建输出表格的列标题，第一列为 time，后续为 node_N。
func buildHeaders(nodeIDs []int) []string {
	headers := make([]string, 1+len(nodeIDs))
	headers[0] = "time"
	for i, rawID := range nodeIDs {
		headers[i+1] = fmt.Sprintf("node_%d", rawID)
	}
	return headers
}

func (r tableRow) toCells() []string {
	cells := make([]string, 1+len(r.vals))
	cells[0] = r.time
	copy(cells[1:], r.vals)
	return cells
}

// formatFloat 以科学记数法（e 格式，最短表示）格式化浮点数。
func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'e', -1, 64)
}

// writeTable 以 ASCII 表格格式输出数据行，自动计算列宽并添加分隔线。
func writeTable(w io.Writer, headers []string, rows []tableRow) {
	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}
	for _, row := range rows {
		cells := row.toCells()
		for i, cell := range cells {
			if len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	sep := buildSeparator(colWidths)

	fmt.Fprintln(w, sep)
	writeRow(w, headers, colWidths)
	fmt.Fprintln(w, sep)

	for _, row := range rows {
		writeRow(w, row.toCells(), colWidths)
	}
	fmt.Fprintln(w, sep)
}

// buildSeparator 根据列宽构建表格分隔行，格式为 +---+---+...。
func buildSeparator(widths []int) string {
	var sb strings.Builder
	sb.WriteByte('+')
	for _, w := range widths {
		sb.WriteString(strings.Repeat("-", w+2))
		sb.WriteByte('+')
	}
	return sb.String()
}

// writeRow 输出表格的一行数据，按列宽对齐填充空白。
func writeRow(w io.Writer, cells []string, widths []int) {
	var sb strings.Builder
	sb.WriteByte('|')
	for i, cell := range cells {
		sb.WriteByte(' ')
		sb.WriteString(cell)
		sb.WriteString(strings.Repeat(" ", widths[i]-len(cell)+1))
		sb.WriteByte('|')
	}
	fmt.Fprintln(w, sb.String())
}
