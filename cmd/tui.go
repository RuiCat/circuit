package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"circuit/element"
	etime "circuit/element/time"
	"circuit/mna"
	"circuit/utils/doublebuffer"
)

var tuiCommands = []string{
	"v", "t", "run", "step", "curve", "trigger", "plot",
	"set", "pause", "resume", "stop", "status", "help", "quit",
}

// ===== 样式 =====
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#4A90D9")).
			Padding(0, 1).
			Width(80)

	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#666666")).
			Padding(0, 1).
			Width(30)

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#666666"))

	triggerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6600")).
			Bold(true)
)

// ===== keyMap =====
type keyMap struct {
	Up    key.Binding
	Down  key.Binding
	PgUp  key.Binding
	PgDn  key.Binding
	Home  key.Binding
	End   key.Binding
	Enter key.Binding
	Tab   key.Binding
	Esc   key.Binding
	Quit  key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Enter, k.Esc}
}
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Tab, k.Up, k.Down, k.Enter, k.Esc}}
}

var keys = keyMap{
	Up:    key.NewBinding(key.WithKeys("up"), key.WithHelp("↑↓", "历史")),
	Down:  key.NewBinding(key.WithKeys("down"), key.WithHelp("PgUp/Dn", "滚动")),
	PgUp:  key.NewBinding(key.WithKeys("pgup"), key.WithHelp("", "")),
	PgDn:  key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("", "")),
	Home:  key.NewBinding(key.WithKeys("home"), key.WithHelp("", "")),
	End:   key.NewBinding(key.WithKeys("end"), key.WithHelp("", "")),
	Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "执行")),
	Tab:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "补全")),
	Esc:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "退出")),
	Quit:  key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("Ctrl+C", "退出")),
}

// ===== simUpdate =====
type simUpdate struct {
	voltages    []float64
	time        float64
	steps       int
	currentStep float64
}

// ===== tuiModel =====
type tuiModel struct {
	con     *element.Context
	timeMNA *etime.TimeMNA
	bufCfg  doublebuffer.Buffer[float64]

	mu          sync.RWMutex
	voltages    []float64
	simTime     float64
	steps       int
	currentStep float64

	triggerNode    int
	triggerOp      string
	triggerValue   float64
	triggerEnabled bool

	input    textinput.Model
	viewport viewport.Model
	help     help.Model

	output  []string
	simDone chan error

	history []string
	histIdx int

	updateCh      chan simUpdate
	quitting      bool
	width         int
	height        int
	vpHeight      int
	focusViewport bool // 焦点在 viewport 时为 true，在输入框时为 false
	lastOutLen    int
}

func newTUIModel(con *element.Context, timeMNA *etime.TimeMNA, bufCfg doublebuffer.Buffer[float64],
	updateCh chan simUpdate, simDone chan error) tuiModel {

	ti := textinput.New()
	ti.Placeholder = "输入命令..."
	ti.Focus()
	ti.Prompt = "> "
	ti.CharLimit = 256

	vp := viewport.New(80, 10)
	vp.Style = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#666666")).Padding(0, 1)

	h := help.New()

	return tuiModel{
		con:      con,
		timeMNA:  timeMNA,
		bufCfg:   bufCfg,
		input:    ti,
		viewport: vp,
		help:     h,
		simDone:  simDone,
		updateCh: updateCh,
		history:  make([]string, 0, 100),
		histIdx:  -1,
	}
}

func (m *tuiModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.listenUpdates())
}

func (m *tuiModel) listenUpdates() tea.Cmd {
	return func() tea.Msg {
		u, ok := <-m.updateCh
		if !ok {
			return nil
		}
		return u
	}
}

// ===== Update =====
func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		vpW := msg.Width - 38
		if vpW < 20 {
			vpW = 20
		}
		m.viewport.Width = vpW
		vpH := msg.Height - 5
		if vpH < 5 {
			vpH = 5
		}
		m.viewport.Height = vpH
		m.vpHeight = vpH
		// input 宽度 = 终端宽 - 边框(2)
		m.input.Width = msg.Width - 4
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			m.timeMNA.Stop()
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, keys.Tab):
			m.handleTabComplete()
			return m, nil

		// ↑：浏览历史命令（无历史时切到 viewport）
		case key.Matches(msg, keys.Up):
			if m.focusViewport {
				m.viewport.LineUp(1)
				return m, nil
			}
			if len(m.history) > 0 {
				if m.histIdx == -1 {
					m.histIdx = len(m.history) - 1
				} else if m.histIdx > 0 {
					m.histIdx--
				}
				m.input.SetValue(m.history[m.histIdx])
				m.input.CursorEnd()
				return m, nil
			}
			if m.input.Value() == "" {
				m.focusViewport = true
			}
			return m, nil

		// ↓：浏览历史命令（首次按下跳到最新一条）
		case key.Matches(msg, keys.Down):
			if m.focusViewport {
				m.viewport.LineDown(1)
				return m, nil
			}
			if len(m.history) > 0 && m.histIdx == -1 {
				m.histIdx = len(m.history) - 1
				m.input.SetValue(m.history[m.histIdx])
				m.input.CursorEnd()
				return m, nil
			}
			if m.histIdx >= 0 {
				if m.histIdx < len(m.history)-1 {
					m.histIdx++
					m.input.SetValue(m.history[m.histIdx])
				} else {
					m.histIdx = -1
					m.input.SetValue("")
				}
				m.input.CursorEnd()
			}
			return m, nil

		case key.Matches(msg, keys.Enter):
			if m.focusViewport {
				m.focusViewport = false
				return m, nil
			}
			line := strings.TrimSpace(m.input.Value())
			m.input.SetValue("")
			m.histIdx = -1
			if line != "" {
				m.history = append(m.history, line)
				if len(m.history) > 100 {
					m.history = m.history[1:]
				}
				m.executeCommand(line)
			}
			return m, nil

		case key.Matches(msg, keys.Esc):
			if m.focusViewport {
				m.focusViewport = false
				return m, nil
			}

		case key.Matches(msg, keys.PgUp):
			m.viewport.PageUp()
			return m, nil
		case key.Matches(msg, keys.PgDn):
			m.viewport.PageDown()
			return m, nil
		case key.Matches(msg, keys.Home):
			m.viewport.GotoTop()
			return m, nil
		case key.Matches(msg, keys.End):
			m.viewport.GotoBottom()
			return m, nil
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.MouseMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case simUpdate:
		m.mu.Lock()
		m.voltages = msg.voltages
		m.simTime = msg.time
		m.steps = msg.steps
		m.currentStep = msg.currentStep
		m.mu.Unlock()
		cmds = append(cmds, m.listenUpdates())
	}

	m.viewport, _ = m.viewport.Update(msg)
	return m, tea.Batch(cmds...)
}

// ===== View =====
func (m *tuiModel) View() string {
	if m.quitting {
		m.timeMNA.Stop()
		return "仿真已停止。\n"
	}

	// --- 标题栏 ---
	st := m.timeMNA.Status()
	statusStr := "运行中"
	switch st {
	case etime.StatusPaused:
		statusStr = "仿真已暂停"
	case etime.StatusStopped:
		statusStr = "已停止"
	case etime.StatusStepping:
		statusStr = "单步中"
	}

	m.mu.RLock()
	t := m.simTime
	s := m.steps
	cs := m.currentStep
	v := m.voltages
	m.mu.RUnlock()

	focusStr := "[输入]"
	if m.focusViewport {
		focusStr = "[浏览]"
	}
	titleContent := fmt.Sprintf(" circuit  │  t=%.4e s  │  dt=%.2e s  │  %s  │  步数:%d", t, cs, statusStr, s)
	contentWidth := m.width - 4 // titleStyle.Width(m.width-2) 减去 Padding(0,1) 的左右各1
	titleLen := lipgloss.Width(titleContent)
	focusLen := lipgloss.Width(focusStr)
	spacer := contentWidth - titleLen - focusLen
	if spacer < 1 {
		spacer = 1
	}
	titleFull := titleContent + strings.Repeat(" ", spacer) + focusStr
	title := titleStyle.Width(m.width - 2).Render(titleFull)

	// --- 主视口内容 ---
	var mainBuf strings.Builder

	// 触发
	if m.triggerEnabled {
		mainBuf.WriteString(triggerStyle.Render(
			fmt.Sprintf("⚡ 触发: node_%d %s %v", m.triggerNode, m.triggerOp, m.triggerValue)) + "\n\n")
	}

	// 输出历史
	if len(m.output) > 0 {
		for i := 0; i < len(m.output); i++ {
			mainBuf.WriteString(m.output[i] + "\n")
		}
	} else if !m.triggerEnabled {
		mainBuf.WriteString("等待仿真数据...\n")
	}

	m.viewport.SetContent(mainBuf.String())
	if len(m.output) != m.lastOutLen {
		m.viewport.GotoBottom()
		m.lastOutLen = len(m.output)
	}

	// --- 侧边栏 ---
	var side strings.Builder

	// 上半：状态信息
	side.WriteString("── 状态 ──\n")
	side.WriteString(fmt.Sprintf(" 状态  %s\n", statusStr))
	side.WriteString(fmt.Sprintf(" 步长  %.2e s\n", cs))
	side.WriteString(fmt.Sprintf(" 步数  %d\n", s))
	side.WriteString(fmt.Sprintf(" 节点  %d\n", len(m.con.CompactNodeID)))
	side.WriteString(fmt.Sprintf(" 元件  %d\n", len(m.con.Nodelist)))
	side.WriteString(fmt.Sprintf(" 缓冲  %d行 %d/%d块\n\n", m.bufCfg.TotalRows(), m.bufCfg.BlockCount(), m.bufCfg.MaxBlocks()))

	// 下半：节点电压表格
	side.WriteString("── 节点电压 ──\n")
	if len(v) > 0 {
		type ni struct {
			rawID int
			val   float64
		}
		var nodes []ni
		for rawID, ci := range m.con.CompactNodeID {
			if ci < len(v) {
				nodes = append(nodes, ni{int(rawID), v[ci]})
			}
		}
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].rawID < nodes[j].rawID })
		// 表头
		side.WriteString(" N#    电压(V)\n")
		side.WriteString(" ──── ──────────\n")
		for _, n := range nodes {
			side.WriteString(fmt.Sprintf(" %-4d %+.4e\n", n.rawID, n.val))
		}
	} else {
		side.WriteString(" 等待数据...\n")
	}

	// --- 组合 ---
	// 让侧边栏高度匹配 viewport: 截断或填充换行
	sideContent := side.String()
	vpLines := m.vpHeight - 2
	if vpLines < 1 {
		vpLines = 1
	}
	curLines := strings.Count(sideContent, "\n") + 1
	if curLines > vpLines {
		// 截断过长内容，防止 JoinHorizontal 撑高布局溢出终端
		lines := strings.SplitN(sideContent, "\n", vpLines+1)
		sideContent = strings.Join(lines[:vpLines], "\n")
	} else if curLines < vpLines {
		sideContent += strings.Repeat("\n", vpLines-curLines)
	}
	sideView := sidebarStyle.Render(sideContent)
	middle := lipgloss.JoinHorizontal(lipgloss.Top, m.viewport.View(), sideView)
	mw := lipgloss.Width(middle)
	if mw < 40 {
		mw = 40
	}
	if m.focusViewport {
		m.input.Placeholder = "浏览模式 (按 Esc 返回输入)"
		m.input.Blur()
	} else {
		m.input.Placeholder = "输入命令 (Tab补全 ↑↓历史)"
		m.input.Focus()
	}
	bottom := inputStyle.Width(mw - 2).Render(m.input.View())

	return title + "\n" + middle + "\n" + bottom
}

// ===== Tab 补全 =====
func (m *tuiModel) handleTabComplete() {
	val := m.input.Value()
	if val == "" {
		return
	}
	var matches []string
	for _, c := range tuiCommands {
		if strings.HasPrefix(c, val) {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		return
	}
	if len(matches) == 1 {
		m.input.SetValue(matches[0] + " ")
	} else {
		prefix := matches[0]
		for _, mc := range matches[1:] {
			for !strings.HasPrefix(mc, prefix) {
				prefix = prefix[:len(prefix)-1]
			}
		}
		if prefix != val {
			m.input.SetValue(prefix)
		} else {
			m.output = append(m.output, strings.Join(matches, " | "))
		}
	}
	m.input.CursorEnd()
}

// ===== 命令执行 =====
func (m *tuiModel) executeCommand(line string) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return
	}
	// 分隔线：区分每次命令输出
	sepW := m.viewport.Width - 6
	if sepW < 20 {
		sepW = 20
	}
	m.output = append(m.output, "  "+strings.Repeat("·", sepW))
	m.output = append(m.output, "  > "+line)
	switch strings.ToLower(parts[0]) {
	case "v":
		m.cmdVoltage(parts[1:])
	case "t":
		m.mu.RLock()
		tt := m.simTime
		m.mu.RUnlock()
		m.output = append(m.output, fmt.Sprintf("  当前时间: %.4e s", tt))
	case "run":
		m.cmdRun(parts[1:])
	case "step":
		m.timeMNA.StepOnce()
		m.output = append(m.output, "  单步执行...")
	case "curve":
		m.cmdCurve(parts[1:])
	case "plot":
		m.cmdPlot(parts[1:])
	case "trigger":
		m.cmdTrigger(parts[1:])
	case "set":
		m.cmdSetEvent(parts[1:])
	case "pause":
		m.timeMNA.Pause()
		m.output = append(m.output, "  仿真已暂停")
	case "resume":
		m.timeMNA.Resume()
		m.output = append(m.output, "  仿真已恢复")
	case "stop", "quit":
		m.timeMNA.Stop()
		m.quitting = true
	case "status":
		m.cmdStatus()
	case "help":
		helpHeaders := []string{"命令", "说明"}
		helpRows := [][]string{
			{"v <节点>", "查询节点电压 (v 1,2 或 v all)"},
			{"t", "显示当前仿真时间"},
			{"run <秒>", "前进 N 秒后自动暂停"},
			{"step", "单步执行一步"},
			{"curve <节点|all> <秒> [点数]", "查询历史电压曲线(可选采样点数)"},
			{"plot <节点|all> [文件]", "绘制当前缓冲区全部电压曲线(PNG)"},
			{"trigger <节点> <op> <值>", "设置电压触发条件"},
			{"trigger off", "清除触发条件"},
			{"set <事件> <值>", "设置事件值"},
			{"pause / resume", "暂停 / 恢复仿真"},
			{"status", "显示仿真状态"},
			{"help", "显示此帮助"},
			{"stop / quit", "停止仿真并退出"},
		}
		w := m.viewport.Width - 6
		if w < 40 {
			w = 40
		}
		m.output = appendTable(m.output, helpHeaders, helpRows, w)
		m.output = append(m.output, "  ↑↓ 历史  │  Tab 补全  │  Esc 切换焦点  │  PgUp/PgDn 滚动  │  鼠标滚轮")
	default:
		m.output = append(m.output, fmt.Sprintf("  未知命令: %s (输入 help 查看帮助)", parts[0]))
	}
	if len(m.output) > 1000 {
		m.output = m.output[len(m.output)-1000:]
	}
}

func (m *tuiModel) cmdVoltage(args []string) {
	m.mu.RLock()
	v := m.voltages
	m.mu.RUnlock()
	if v == nil {
		m.output = append(m.output, "  仿真尚未开始，请先 resume 或 run")
		return
	}

	type ni struct {
		rawID int
		val   float64
	}
	var nodes []ni

	if len(args) == 0 || args[0] == "all" {
		for rawID, ci := range m.con.CompactNodeID {
			if ci < len(v) {
				nodes = append(nodes, ni{int(rawID), v[ci]})
			}
		}
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].rawID < nodes[j].rawID })
	} else {
		seen := make(map[int]bool)
		for _, arg := range args {
			for _, ns := range strings.Split(arg, ",") {
				ns = strings.TrimSpace(ns)
				if ns == "" {
					continue
				}
				n, err := strconv.Atoi(ns)
				if err != nil {
					m.output = append(m.output, fmt.Sprintf("  无效节点: %s", ns))
					continue
				}
				if seen[n] {
					continue
				}
				ci, ok := m.con.CompactNodeID[mna.NodeID(n)]
				if !ok || ci >= len(v) {
					m.output = append(m.output, fmt.Sprintf("  节点 %d 不存在", n))
				} else {
					nodes = append(nodes, ni{n, v[ci]})
					seen[n] = true
				}
			}
		}
	}

	if len(nodes) == 0 {
		m.output = append(m.output, "  无节点数据")
		return
	}

	headers := []string{"节点", "电压(V)"}
	var rows [][]string
	for _, n := range nodes {
		rows = append(rows, []string{
			fmt.Sprintf("node_%d", n.rawID),
			fmt.Sprintf("%+.6e", n.val),
		})
	}
	w := m.viewport.Width - 6
	if w < 40 {
		w = 40
	}
	m.output = appendTable(m.output, headers, rows, w)
}

func (m *tuiModel) cmdRun(args []string) {
	if len(args) == 0 {
		m.output = append(m.output, "  用法: run <秒数>")
		return
	}
	d, err := strconv.ParseFloat(args[0], 64)
	if err != nil || d <= 0 {
		m.output = append(m.output, "  错误: 无效的时间值")
		return
	}
	m.timeMNA.AdvanceFor(d)
	m.output = append(m.output, fmt.Sprintf("  前进 %.3e s...", d))
}

func (m *tuiModel) cmdCurve(args []string) {
	if len(args) < 2 {
		m.output = append(m.output, "  用法: curve <节点|all> <秒数> [点数]")
		return
	}
	nodeStr := args[0]
	sec, err := strconv.ParseFloat(args[1], 64)
	if err != nil || sec <= 0 {
		m.output = append(m.output, "  错误: 无效的秒数值")
		return
	}
	pts := 0
	if len(args) >= 3 {
		n, e := strconv.Atoi(args[2])
		if e == nil && n > 0 {
			pts = n
		}
	}

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

	m.mu.RLock()
	total := m.bufCfg.TotalRows()
	rows := m.bufCfg.LastRows(total)
	m.mu.RUnlock()
	if len(rows) == 0 {
		m.output = append(m.output, "  暂无历史数据")
		return
	}

	curT := rows[len(rows)-1][0]
	startT := curT - sec
	bufSpan := curT - rows[0][0]
	if sec > bufSpan {
		m.output = append(m.output, fmt.Sprintf("  ⚠ 请求 %.3e s 但缓冲区仅保留 %.3e s，已截断", sec, bufSpan))
		startT = rows[0][0]
	}

	var filtered [][]float64
	for _, r := range rows {
		if r[0] >= startT {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) == 0 && len(rows) > 0 {
		filtered = [][]float64{rows[len(rows)-1]}
	}

	disp := filtered
	if pts == 1 && len(filtered) > 0 {
		disp = [][]float64{filtered[len(filtered)-1]}
	} else if pts > 1 && pts < len(filtered) {
		disp = make([][]float64, pts)
		step := float64(len(filtered)-1) / float64(pts-1)
		for i := 0; i < pts; i++ {
			idx := int(float64(i) * step)
			if idx >= len(filtered) {
				idx = len(filtered) - 1
			}
			disp[i] = filtered[idx]
		}
	}

	// 自适应宽度表格
	headers := []string{"time"}
	for _, n := range qn {
		headers = append(headers, fmt.Sprintf("node_%d", n))
	}

	var tRows [][]string
	for _, r := range disp {
		row := []string{formatFloat(r[0])}
		for _, n := range qn {
			ci, ok := m.con.CompactNodeID[mna.NodeID(n)]
			var val float64
			if ok && ci+1 < len(r) {
				val = r[ci+1]
			}
			row = append(row, fmt.Sprintf("%.6e", val))
		}
		tRows = append(tRows, row)
	}

	w := m.viewport.Width - 6
	if w < 40 {
		w = 40
	}
	m.output = appendTable(m.output, headers, tRows, w)
	m.output = append(m.output, fmt.Sprintf("(%d 点, 跨度 %.3e s)", len(disp), curT-startT))
}

func (m *tuiModel) cmdTrigger(args []string) {
	if len(args) == 0 || args[0] == "off" {
		m.triggerEnabled = false
		m.triggerNode = -1
		m.output = append(m.output, "  触发条件已清除")
		return
	}
	if len(args) < 3 {
		m.output = append(m.output, "  用法: trigger <节点> <op> <值>  或  trigger off")
		return
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		m.output = append(m.output, "  错误: 无效的节点ID")
		return
	}
	switch args[1] {
	case ">=", "<=", ">", "<", "==":
		m.triggerOp = args[1]
	default:
		m.output = append(m.output, "  错误: 无效操作符 (支持 >= <= > < ==)")
		return
	}
	v, err := strconv.ParseFloat(args[2], 64)
	if err != nil {
		m.output = append(m.output, "  错误: 无效的阈值")
		return
	}
	m.triggerNode = n
	m.triggerValue = v
	m.triggerEnabled = true
	m.output = append(m.output, fmt.Sprintf("  ⚡ 触发条件: node_%d %s %v", n, args[1], v))
}

func (m *tuiModel) cmdSetEvent(args []string) {
	if len(args) < 2 {
		m.output = append(m.output, "  用法: set <事件名> <值>")
		return
	}
	v, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		m.output = append(m.output, "  错误: 无效的值")
		return
	}
	m.con.SetEvent(args[0], v)
	m.output = append(m.output, fmt.Sprintf("  %s = %v", args[0], v))
}

func (m *tuiModel) cmdStatus() {
	st := m.timeMNA.Status()
	ss := "运行中"
	switch st {
	case etime.StatusPaused:
		ss = "  仿真已暂停"
	case etime.StatusStopped:
		ss = "已停止"
	case etime.StatusStepping:
		ss = "单步中"
	}
	m.mu.RLock()
	s := m.steps
	tt := m.simTime
	m.mu.RUnlock()
	m.output = append(m.output, fmt.Sprintf("  %s | %d 步 | t = %.4e s", ss, s, tt))
}

func (m *tuiModel) checkTrigger(v []float64) {
	if !m.triggerEnabled || m.triggerNode < 0 {
		return
	}
	ci, ok := m.con.CompactNodeID[mna.NodeID(m.triggerNode)]
	if !ok || ci >= len(v) {
		return
	}
	nv := v[ci]
	var trig bool
	switch m.triggerOp {
	case ">=":
		trig = nv >= m.triggerValue
	case "<=":
		trig = nv <= m.triggerValue
	case ">":
		trig = nv > m.triggerValue
	case "<":
		trig = nv < m.triggerValue
	case "==":
		trig = nv-m.triggerValue < 1e-9 && m.triggerValue-nv < 1e-9
	}
	if trig {
		m.timeMNA.Pause()
		m.triggerEnabled = false
		m.output = append(m.output, fmt.Sprintf("  ⚡ 已触发! node_%d %s %v (=%e)",
			m.triggerNode, m.triggerOp, m.triggerValue, nv))
	}
}

// ===== runTUI =====
func runTUI(con *element.Context, cfg config) error {
	tm, err := etime.NewTimeMNA(1.0)
	if err != nil {
		return fmt.Errorf("TimeMNA: %w", err)
	}
	con.Time = tm
	con.Time.SetTolerances(cfg.absTol, cfg.relTol)
	con.Time.SetStepLimits(cfg.minStep, cfg.maxStep)
	ei := tm.MaxElemIter()
	con.Time.SetIterationLimits(cfg.maxIter, ei)
	con.Time.SetMaxTimeSteps(cfg.maxSteps)
	con.Time.SetTimeStep(cfg.initialStep)
	if cfg.parallel > 0 {
		con.ParallelOpts = &element.ParallelOptions{StampWorkers: cfg.parallel}
	}

	buf := doublebuffer.NewBuffer[float64](100, con.GetNodeNum()+1)
	buf.SetMaxBlocks(1000)

	tm.SetContinuousMode()
	tm.Pause()

	ch := make(chan simUpdate, 200)
	done := make(chan error, 1)
	m := newTUIModel(con, tm, buf, ch, done)
	m.output = append(m.output, "  连续仿真已就绪（暂停中）。输入 help 查看命令。")

	go func() {
		stepCnt := 0
		call := func(v []float64) {
			m.checkTrigger(v)
			cp := make([]float64, len(v))
			copy(cp, v)
			stepCnt++
			select {
			case ch <- simUpdate{voltages: cp, time: con.CurrentTime(), steps: stepCnt, currentStep: con.CurrentStep()}:
			default:
			}
			row := make([]float64, 1+len(v))
			row[0] = con.CurrentTime()
			copy(row[1:], v)
			buf.Append(row)
		}
		done <- etime.TransientSimulation(con, call)
	}()

	p := tea.NewProgram(&m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return err
	}
	tm.Stop()
	<-done
	return nil
}
