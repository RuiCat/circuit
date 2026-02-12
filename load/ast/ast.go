// Package ast 提供电路网表解析的抽象语法树（AST）功能。
// 它能够解析包含元件定义、值设置命令和注释的电路网表文本，
// 并构建相应的语法树结构供后续处理使用。
package ast

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

// 常量定义 - 用于词法分析和语法分析的关键字和符号
const (
	tokenValue             = ".value" // 值设置命令
	tokenNewline           = "\n"     // 换行符
	tokenSpace             = " "      // 空格
	tokenTab               = "	"      // 制表符
	tokenLBracket          = "["      // 左方括号
	tokenRBracket          = "]"      // 右方括号
	tokenComma             = ","      // 逗号分隔符
	tokenCommentHash       = "#"      // # 注释
	tokenCommentLine       = "//"     // // 行注释
	tokenCommentBlockStart = "/*"     // /* 块注释开始
	tokenCommentBlockEnd   = "*/"     // */ 块注释结束
)

// ElementNode 表示元件定义节点
type ElementNode struct {
	Type   string  // 元件类型，如 "v", "r", "c"
	ID     string  // 元件ID，如 "1"
	Pins   []Value // 引脚列表
	Values []Value // 值列表
	Line   int     // 行号
}

// ValueNode 表示值设置节点
type ValueNode struct {
	Command string // 命令，如 ".value"
	Name    string // 变量名
	Value   Value  // 值
	Line    int    // 行号
}

// CommentNode 表示注释节点
type CommentNode struct {
	Text string // 注释文本
	Line int    // 行号
}

// ParseTree 解析树
type ParseTree struct {
	ElementNodes []*ElementNode    // 元件列表
	CommentNodes []*CommentNode    // 注释列表
	ValueNodes   map[string]string // 变量列表
}

// String 打印
func (parseTree *ParseTree) String() {
	// 打印解析树用于调试
	fmt.Printf("解析成功! 找到 %d 个元件, %d 个值设置, %d 个注释\n",
		len(parseTree.ElementNodes), len(parseTree.ValueNodes), len(parseTree.CommentNodes))
	for i, n := range parseTree.ElementNodes {
		fmt.Printf("\n元件 %d: %s%s (行号: %d)\n", i+1, n.Type, n.ID, n.Line)
		fmt.Printf("  引脚 (%d 个): ", len(n.Pins))
		for j := range n.Pins {
			fmt.Printf("%s(variable:%v) ", n.Pins[j].Value, n.Pins[j].IsVar)
			if j < len(n.Pins)-1 {
				fmt.Printf(", ")
			}
		}
		fmt.Printf("\n")
		fmt.Printf("  值 (%d 个): ", len(n.Values))
		for j := range n.Values {
			fmt.Printf("%s(variable:%v) ", n.Values[j].Value, n.Values[j].IsVar)
			if j < len(n.Values)-1 {
				fmt.Printf(", ")
			}
		}
		fmt.Printf("\n")
	}
	for Name := range parseTree.ValueNodes {
		fmt.Printf("\n值设置: %s = %s\n", Name, parseTree.ValueNodes[Name])
	}
	for i := range parseTree.CommentNodes {
		fmt.Printf("\n注释 %d (行号: %d): %s\n", i+1, parseTree.CommentNodes[i].Line, parseTree.CommentNodes[i].Text)
	}
}

// NewParseTree 生成网表解析树
func NewParseTree(r io.Reader) (parseTree *ParseTree, err error) {
	return NewParseTreeDirect(r)
}

// NewParseTreeDirect 直接生成网表解析树（流式处理，不先收集 tokens）
func NewParseTreeDirect(r io.Reader) (parseTree *ParseTree, err error) {
	scanner := bufio.NewScanner(r)
	scanner.Split(SplitTokens)
	// 创建解析树
	parseTree = &ParseTree{
		ValueNodes: map[string]string{},
	}
	lineNum := 1
	var pendingToken *string = nil
	for {
		var token string
		if pendingToken != nil {
			token = *pendingToken
			pendingToken = nil
		} else {
			if !scanner.Scan() {
				break
			}
			token = scanner.Text()
		}
		// 处理换行符
		if token == tokenNewline {
			lineNum++
			continue
		}
		// 跳过空格和制表符
		if token == tokenSpace || token == tokenTab {
			continue
		}
		// 处理注释
		if parseComment(token, lineNum, parseTree) {
			continue
		}
		// 处理 .value 命令
		if token == tokenValue {
			if err := parseValueCommandFromScanner(scanner, lineNum, parseTree); err != nil {
				return nil, err
			}
			continue
		}
		// 处理元件定义
		if len(token) > 0 && isLetter(token[0]) {
			hasMore, err := parseElementDefinitionFromScanner(scanner, token, lineNum, parseTree)
			if err != nil {
				return nil, err
			}
			// 如果还有未处理的 token，保存它供下一次循环处理
			if hasMore && scanner.Scan() {
				nextToken := scanner.Text()
				pendingToken = &nextToken
			}
			continue
		}
		// 未知 token，忽略（可能是数字或其他符号）
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取网表时出错: %w", err)
	}
	return parseTree, nil
}

// parseValueListFromScanner 从 scanner 解析值列表
func parseValueListFromScanner(scanner *bufio.Scanner, lineNum int) ([]Value, error) {
	var values []Value
	for scanner.Scan() {
		token := scanner.Text()
		// 如果遇到 ]，表示列表结束
		if token == tokenRBracket {
			break
		}
		// 跳过逗号分隔符、空格和制表符
		if token == tokenComma || token == tokenSpace || token == tokenTab {
			continue
		}
		// 处理变量（以 % 开头）
		isVar := false
		valueStr := token
		if len(token) > 0 && token[0] == '%' {
			isVar = true
			valueStr = token[1:]
		}
		values = append(values, Value{
			Value: valueStr,
			Line:  lineNum,
			IsVar: isVar,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取值列表时出错: %w", err)
	}
	return values, nil
}

// parseComment 解析注释 token
func parseComment(token string, lineNum int, parseTree *ParseTree) bool {
	switch {
	case token[0] == tokenCommentHash[0]:
		comment := token[1:]
		parseTree.CommentNodes = append(parseTree.CommentNodes, &CommentNode{
			Text: comment,
			Line: lineNum,
		})
		return true
	case len(token) < 2:
		return false
	case token[0:2] == tokenCommentLine:
		comment := token[2:]
		parseTree.CommentNodes = append(parseTree.CommentNodes, &CommentNode{
			Text: comment,
			Line: lineNum,
		})
		return true
	case token[0:2] == tokenCommentBlockStart:
		comment := token[2:]
		if len(comment) >= 2 && comment[len(comment)-2:] == tokenCommentBlockEnd {
			comment = comment[:len(comment)-2]
		}
		parseTree.CommentNodes = append(parseTree.CommentNodes, &CommentNode{
			Text: comment,
			Line: lineNum,
		})
		return true
	}
	return false
}

// parseValueCommandFromScanner 从 scanner 解析 .value 命令
func parseValueCommandFromScanner(scanner *bufio.Scanner, lineNum int, parseTree *ParseTree) error {
	// 读取名称 token，跳过空格和制表符
	var name string
	for {
		if !scanner.Scan() {
			return errorAtLine(lineNum, ".value 命令缺少名称")
		}
		token := scanner.Text()
		if token == tokenSpace || token == tokenTab {
			continue
		}
		name = token
		break
	}
	// 读取值 token，跳过空格和制表符
	var valueStr string
	for {
		if !scanner.Scan() {
			return errorAtLine(lineNum, ".value 命令缺少值")
		}
		token := scanner.Text()
		if token == tokenSpace || token == tokenTab {
			continue
		}
		valueStr = token
		break
	}
	parseTree.ValueNodes[name] = valueStr
	return nil
}

// errorAtLine 生成带行号的错误信息
func errorAtLine(lineNum int, format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("第 %d 行: %s", lineNum, msg)
}

// parseElementDefinitionFromScanner 从 scanner 解析元件定义
func parseElementDefinitionFromScanner(scanner *bufio.Scanner, elementType string, lineNum int, parseTree *ParseTree) (bool, error) {
	// 读取元件ID，跳过空格和制表符
	var elementID string
	for {
		if !scanner.Scan() {
			return false, errorAtLine(lineNum, "缺少元件 ID")
		}
		token := scanner.Text()
		if token == tokenSpace || token == tokenTab {
			continue
		}
		if !isNumber(token) {
			return false, errorAtLine(lineNum, "元件 ID 必须是数字")
		}
		elementID = token
		break
	}
	// 读取引脚列表开始标记，跳过空格和制表符
	for {
		if !scanner.Scan() {
			return false, errorAtLine(lineNum, "缺少引脚列表")
		}
		token := scanner.Text()
		if token == tokenSpace || token == tokenTab {
			continue
		}
		if token != tokenLBracket {
			return false, errorAtLine(lineNum, "缺少引脚列表开始标记 [")
		}
		break
	}

	// 解析引脚列表
	pins, err := parseValueListFromScanner(scanner, lineNum)
	if err != nil {
		return false, err
	}
	// parseValueListFromScanner 已经消耗了 ]，所以这里不需要再读取
	// 解析可选的值列表
	var values []Value
	hasMore := false
	// 跳过空格和制表符，检查是否有值列表
	for {
		if !scanner.Scan() {
			// 没有更多 token
			hasMore = false
			break
		}
		token := scanner.Text()
		if token == tokenSpace || token == tokenTab {
			continue
		}
		if token == tokenLBracket {
			// 找到值列表开始标记
			values, err = parseValueListFromScanner(scanner, lineNum)
			if err != nil {
				return false, err
			}
			// parseValueListFromScanner 已经消耗了 ]，所以这里不需要再检查
			hasMore = scanner.Scan()
			break
		} else {
			// 不是值列表，保存这个 token 供后续处理
			hasMore = true
			// 注意：这里我们有一个 token 没有被消耗，需要在调用者中处理
			break
		}
	}
	parseTree.ElementNodes = append(parseTree.ElementNodes, &ElementNode{
		Type:   elementType,
		ID:     elementID,
		Pins:   pins,
		Values: values,
		Line:   lineNum,
	})
	// 返回是否还有未处理的 token
	return hasMore, nil
}

// isLetter 检查是否是字母
func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

// isNumber 检查字符串是否表示数字
func isNumber(s string) bool {
	if len(s) == 0 {
		return false
	}
	// 检查第一个字符，如果是数字或负号开头
	if s[0] == '-' || s[0] == '+' {
		if len(s) > 1 {
			return s[1] >= '0' && s[1] <= '9'
		}
		return false
	}
	return s[0] >= '0' && s[0] <= '9'
}

// SplitTokens 分割标识符
func SplitTokens(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i := range data {
		switch data[i] {
		case '#':
			return bufio.ScanLines(data, atEOF)
		case '1', '2', '3', '4', '5', '6', '7', '8', '9', '0', '+', '-':
			if i != 0 {
				return i, data[:i], nil
			}
			for i < len(data) {
				c := data[i]
				if (c >= '0' && c <= '9') || c == '.' || c == 'e' || c == 'E' ||
					c == 'i' || c == 'j' || c == '+' || c == '-' {
					i++
					continue
				}
				break
			}
			return i, data[:i], nil
		case '/':
			if len(data) > i+1 {
				switch data[i+1] {
				case '*':
					if i := bytes.Index(data, []byte("*/")); i >= 0 {
						i += 2
						return i, data[0:i], nil
					}
				case '/':
					return bufio.ScanLines(data, atEOF)
				}
			}
			return i + 1, data[0 : i+1], nil
		case ' ', '	', '\n', ',', ']', '[':
			if i != 0 {
				return i, data[:i], nil
			}
			return i + 1, data[0 : i+1], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
