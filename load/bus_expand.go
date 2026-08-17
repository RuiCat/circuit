package load

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// 正则表达式编译（包级别，只编译一次）
var (
	rangeBusRe  = regexp.MustCompile(`\b([A-Za-z_]\w{0,63})\[(\d{1,10}):(\d{1,10})\]\B`)
	singleBusRe = regexp.MustCompile(`\b([A-Za-z_]\w{0,63})\[(\d{1,10})\]\B`)
)

// maxBusWidth 单条总线最大展开宽度。
const maxBusWidth = 4096

// maxExpandedSize 展开后网表文本的最大累计字节数，防止总线展开放大攻击导致 OOM。
const maxExpandedSize = 200 * 1024 * 1024 // 200MB

// ExpandBusNotation 将网表文本中的总线表示法展开为独立信号线。
// 支持两种模式：
//   - name[msb:lsb] 范围总线 → name<msb> name<msb±1> ... name<lsb>（降序或升序）
//   - name[index]   单索引总线 → name<index>
// 返回展开后的网表文本。
func ExpandBusNotation(text string) (string, error) {
	// 统一行尾符（处理 \r\n 和 \r）
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	var result strings.Builder
	result.Grow(len(text))

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i > 0 {
			result.WriteByte('\n')
		}
		processed, err := processLine(line)
		if err != nil {
			return "", err
		}
		result.WriteString(processed)
		// 增量检查累计大小，在展开完成前及时终止，防止大量总线展开导致 OOM。
		if result.Len() > maxExpandedSize {
			return "", fmt.Errorf("展开后网表过大（超过 %d 字节）", maxExpandedSize)
		}
	}
	return result.String(), nil
}

// processLine 处理单行文本，分离注释并展开总线。
func processLine(line string) (string, error) {
	trimmed := strings.TrimSpace(line)

	// 跳过纯注释行（以 # 或 // 开头）
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
		return line, nil
	}

	// 分离代码部分和行尾注释
	codePart, commentPart := splitLineComment(line)

	// 先展开范围总线，再展开单索引总线。
	// 注意顺序不可交换：singleBusRe 会错误匹配范围总线中的前缀部分
	// （如 data[3:0] 中的 data[3]），必须先由 rangeBusRe 整体消费。
	var err error
	codePart = rangeBusRe.ReplaceAllStringFunc(codePart, func(match string) string {
		sub := rangeBusRe.FindStringSubmatch(match)
		if len(sub) != 4 {
			return match
		}
		expanded, e := expandBusRange(sub[1], sub[2], sub[3])
		if e != nil {
			err = e
			return match
		}
		return expanded
	})
	if err != nil {
		return "", err
	}

	codePart = singleBusRe.ReplaceAllStringFunc(codePart, func(match string) string {
		sub := singleBusRe.FindStringSubmatch(match)
		if len(sub) != 3 {
			return match
		}
		return sub[1] + sub[2]
	})

	return codePart + commentPart, nil
}

// splitLineComment 将行分为代码部分和注释部分。
// 注释以 # 或 // 开始。
func splitLineComment(line string) (code, comment string) {
	inQuote := false
	for i := 0; i < len(line); i++ {
		if line[i] == '"' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			continue
		}
		if line[i] == '#' {
			return line[:i], line[i:]
		}
		if line[i] == '/' && i+1 < len(line) && line[i+1] == '/' {
			return line[:i], line[i:]
		}
	}
	return line, ""
}

// expandBusRange 将 bus_name、msbStr、lsbStr 展开为空格分隔的信号名列表。
func expandBusRange(name, msbStr, lsbStr string) (string, error) {
	msb, err := strconv.Atoi(msbStr)
	if err != nil {
		return "", fmt.Errorf("总线 '%s' 的 MSB '%s' 不是有效数字", name, msbStr)
	}
	lsb, err := strconv.Atoi(lsbStr)
	if err != nil {
		return "", fmt.Errorf("总线 '%s' 的 LSB '%s' 不是有效数字", name, lsbStr)
	}

	width := 1
	if msb >= lsb {
		width = msb - lsb + 1
	} else {
		width = lsb - msb + 1
	}

	if width > maxBusWidth {
		return "", fmt.Errorf("总线 '%s[%s:%s]' 宽度 %d 超过上限 %d",
			name, msbStr, lsbStr, width, maxBusWidth)
	}

	var names []string
	if msb >= lsb {
		for i := msb; i >= lsb; i-- {
			names = append(names, name+strconv.Itoa(i))
		}
	} else {
		for i := msb; i <= lsb; i++ {
			names = append(names, name+strconv.Itoa(i))
		}
	}
	return strings.Join(names, " "), nil
}
