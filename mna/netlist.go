package mna

import "strconv"

// NetList 网表定义
type NetList []string

func (vlaue NetList) ParseFloat(i int, defaultValue float64) float64 {
	if i < len(vlaue) {
		if val, err := strconv.ParseFloat(vlaue[i], 64); err == nil {
			return val
		}
	}
	return defaultValue
}

func (vlaue NetList) ParseInt(i int, defaultValue int) int {
	if i < len(vlaue) {
		if val, err := strconv.Atoi(vlaue[i]); err == nil {
			return val
		}
	}
	return defaultValue
}
