package mna

import "strconv"

// NetList 网表定义
type NetList []string

// ParseFloat64 解析浮点数
func (vlaue NetList) ParseFloat64(i int, defaultValue float64) float64 {
	if i < len(vlaue) {
		if val, err := strconv.ParseFloat(vlaue[i], 64); err == nil {
			return val
		}
	}
	return defaultValue
}

// ParseInt 解析整数
func (vlaue NetList) ParseInt(i int, defaultValue int) int {
	if i < len(vlaue) {
		if val, err := strconv.Atoi(vlaue[i]); err == nil {
			return val
		}
	}
	return defaultValue
}
