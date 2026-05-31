package main

// 本文件提供 TUI 中自适应宽度表格的渲染工具函数，包括动态列宽计算和输出格式化。

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// renderTable 创建并渲染自适应宽度表格，根据终端宽度动态分配列宽。
// Column widths are calculated based on content and available width.
// Returns the rendered table string, or "" if rows is empty.
func renderTable(headers []string, rows [][]string, width int) string {
	if len(rows) == 0 {
		return ""
	}

	ncols := len(headers)
	minColWidth := 5

	// Calculate max content width per column
	contentWidths := make([]int, ncols)
	for i, h := range headers {
		contentWidths[i] = len(h)
	}
	for _, row := range rows {
		for i := 0; i < ncols && i < len(row); i++ {
			if len(row[i]) > contentWidths[i] {
				contentWidths[i] = len(row[i])
			}
		}
	}

	// Add padding (1 left + 1 right)
	colWidths := make([]int, ncols)
	totalNatural := 0
	for i, cw := range contentWidths {
		w := max(cw+2, minColWidth)
		colWidths[i] = w
		totalNatural += w
	}

	// Distribute available width
	if totalNatural < width {
		// Expand proportionally
		extra := width - totalNatural
		for i := range ncols {
			share := extra / (ncols - i)
			colWidths[i] += share
			extra -= share
		}
	} else if totalNatural > width {
		// Shrink proportionally, keep minimum
		scale := float64(width) / float64(totalNatural)
		allocated := 0
		for i := 0; i < ncols-1; i++ {
			w := max(int(float64(colWidths[i])*scale), minColWidth)
			colWidths[i] = w
			allocated += w
		}
		colWidths[ncols-1] = max(width-allocated, minColWidth)
	}

	// Build table columns with calculated widths
	cols := make([]table.Column, ncols)
	for i, h := range headers {
		cols[i] = table.Column{Title: h, Width: colWidths[i]}
	}

	// Build table rows
	tRows := make([]table.Row, len(rows))
	for i, row := range rows {
		tRows[i] = table.Row(row)
	}

	// Styles
	s := table.DefaultStyles()
	s.Header = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	s.Cell = lipgloss.NewStyle().Padding(0, 1)
	s.Selected = lipgloss.NewStyle()

	height := len(rows) + 1 // header + all data
	if height < 2 {
		height = 2
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(tRows),
		table.WithWidth(width),
		table.WithHeight(height),
		table.WithStyles(s),
	)

	return t.View()
}

// appendTable 将格式化表格追加到输出日志。
func appendTable(out []string, headers []string, rows [][]string, width int) []string {
	tableStr := renderTable(headers, rows, width)
	if tableStr == "" {
		return out
	}
	lines := strings.Split(strings.TrimRight(tableStr, "\n"), "\n")
	for _, line := range lines {
		out = append(out, "  "+line)
	}
	return out
}
