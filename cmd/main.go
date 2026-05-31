// Command circuit 是电路仿真命令行工具，支持批处理仿真和交互式 TUI 连续仿真两种模式。基于 bubbletea 的终端界面提供实时电压监控、事件设置和曲线绘制功能。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	_ "circuit/element/base"
	"circuit/element"
	"circuit/element/time"
	"circuit/load"
	"circuit/mna"
)


// errWriter 包装 io.Writer，记录首次写入错误。
// 后续写入自动丢弃，避免静默数据丢失。
type errWriter struct {
	w   io.Writer
	err error
}

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
// 支持批处理和交互式两种模式。
func main() {
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

	if cfg.format != "csv" && cfg.format != "tsv" && cfg.format != "table" {
		fmt.Fprintf(os.Stderr, "错误: 不支持的输出格式 '%s'，可选: csv, tsv, table\n", cfg.format)
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

	flag.StringVar(&cfg.format, "format", "csv", "输出格式: csv, tsv, table")

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

// printHelp 输出命令行帮助信息到 stderr。
func printHelp() {
	fmt.Fprint(os.Stderr, `circuit - 电路仿真命令行工具
用法:
  circuit [选项] <网表文件>             批处理仿真
  circuit [选项] interactive <网表文件>  交互式连续仿真

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
      --format string  输出格式: csv, tsv, table（默认: csv）
      --quiet          静默模式，不输出元信息到 stderr
  -h, --help           显示帮助

示例:
  circuit circuit.net
  circuit -t 0.01 --nodes 1,2,5 -o result.csv circuit.net
  circuit --parallel 4 --format table circuit.net
`)
}

// maxTableRows 限制终端表格输出的最大行数，超出后自动降级为 csv 格式。
const maxTableRows = 10000

// tableRow 表示表格输出的一行数据，包含时间和多个节点电压值。
type tableRow struct {
	time string
	vals []string
}

// runSim 执行完整的批处理仿真流程，包括时间控制器设置、节点筛选、步进执行和结果输出。
func runSim(w io.Writer, con *element.Context, cfg config) error {
	ew := &errWriter{w: w}

	timeMNA, err := time.NewTimeMNA(cfg.targetTime)
	if err != nil {
		return fmt.Errorf("创建时间控制器失败: %w", err)
	}
	con.Time = timeMNA

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

	if !cfg.quiet {
		fmt.Fprintf(os.Stderr, "加载成功: %d 个元件, %d 个节点\n",
			len(con.Nodelist), len(con.CompactNodeID))
		fmt.Fprintf(os.Stderr, "仿真开始: 目标时间 %.3e 秒\n", cfg.targetTime)
	}

	var tableRows []tableRow
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
		}
		stepCount++
	}

	if err := time.TransientSimulation(con, call); err != nil {
		return fmt.Errorf("仿真失败: %w", err)
	}

	if ew.err != nil {
		return fmt.Errorf("输出写入失败: %w", ew.err)
	}

	if cfg.format == "table" {
		writeTable(ew, headers, tableRows)
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

