package mcp

import (
	"circuit/element"
	"fmt"
	"strconv"
	"strings"
)

// elementNode 元件节点接口（别名）。
type elementNode = element.NodeFace

// elementNodeRef 包装元件节点，提供类型化参数读写。
type elementNodeRef struct {
	elementNode
}

// Get 读取参数值（越界返回 nil）。
func (r *elementNodeRef) Get(idx int) any {
	base := r.elementNode.Base()
	if idx < 0 || idx >= len(base.NodeValue) {
		return nil
	}
	return base.NodeValue[idx]
}

// Set 按当前槽位类型写入参数值。
func (r *elementNodeRef) Set(idx int, v any) error {
	base := r.elementNode.Base()
	if idx < 0 || idx >= len(base.NodeValue) {
		return fmt.Errorf("参数索引 %d 越界", idx)
	}
	cur := base.NodeValue[idx]
	switch nv := v.(type) {
	case float64:
		if cur != nil {
			if _, ok := cur.(float64); !ok {
				return fmt.Errorf("槽位类型 %T 与 float64 不匹配", cur)
			}
		}
		r.elementNode.SetFloat64(idx, nv)
	case int:
		if cur != nil {
			if _, ok := cur.(int); !ok {
				return fmt.Errorf("槽位类型 %T 与 int 不匹配", cur)
			}
		}
		r.elementNode.SetInt(idx, nv)
	case bool:
		if cur != nil {
			if _, ok := cur.(bool); !ok {
				return fmt.Errorf("槽位类型 %T 与 bool 不匹配", cur)
			}
		}
		r.elementNode.SetBool(idx, nv)
	case string:
		if cur != nil {
			if _, ok := cur.(string); !ok {
				return fmt.Errorf("槽位类型 %T 与 string 不匹配", cur)
			}
		}
		r.elementNode.SetString(idx, nv)
	default:
		return fmt.Errorf("不支持的参数类型 %T", v)
	}
	return nil
}

// coerceParam 将工具传入值强制转换为与旧值匹配的类型。
func coerceParam(v any, old any) (any, error) {
	switch old.(type) {
	case float64:
		switch t := v.(type) {
		case float64:
			return t, nil
		case int:
			return float64(t), nil
		case string:
			f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
			if err != nil {
				return nil, err
			}
			return f, nil
		}
	case int:
		switch t := v.(type) {
		case float64:
			return int(t), nil
		case int:
			return t, nil
		case string:
			n, err := strconv.Atoi(strings.TrimSpace(t))
			if err != nil {
				return nil, err
			}
			return n, nil
		}
	case bool:
		if b, ok := v.(bool); ok {
			return b, nil
		}
	case string:
		if s, ok := v.(string); ok {
			return s, nil
		}
	case nil:
		// 未初始化槽位：按传入类型写入
		switch v.(type) {
		case float64, int, bool, string:
			return v, nil
		}
	}
	return nil, fmt.Errorf("收到类型 %T", v)
}
