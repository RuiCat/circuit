package utils

import (
	"sync"
)

// FlagValue 用于标记数值
type FlagValue uint64

// Flag 通过进行事件管理
// @ 使用方法通过锁定事件获取的标记值通过等待事件函数进行等待.
// @ 如果释放事件被触发则继续指定标记位的等待事件.
// @ 获取状态 用来获取当前事件是否存在等待内容.
// @ 如果释放或等待没有锁定的事件返回为假.
type Flag interface {
	LockEvent() (flag FlagValue, ok bool) // 锁定事件
	WaitEvent(flag FlagValue) bool        // 等待事件
	ReleaseEvent(flag FlagValue) bool     // 释放事件
	GetStatus(flag FlagValue) bool        // 获取状态
}

type flagImpl struct {
	mu       sync.Mutex         // 互斥锁，保护共享数据
	cond     *sync.Cond         // 条件变量，用于等待和通知
	nextFlag FlagValue          // 下一个可用的标记值
	released map[FlagValue]bool // 已释放的标记集合
	waiting  map[FlagValue]bool // 正在等待的标记集合
	reusable []FlagValue        // 可重用的标记值列表
}

// NewFlag 创建一个新的 Flag 实例
func NewFlag() Flag {
	f := &flagImpl{
		released: make(map[FlagValue]bool),
		waiting:  make(map[FlagValue]bool),
		reusable: make([]FlagValue, 0),
	}
	f.cond = sync.NewCond(&f.mu)
	return f
}

// LockEvent 锁定事件，返回一个新的标记值
func (f *flagImpl) LockEvent() (flag FlagValue, ok bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 优先从可重用标记中获取
	if len(f.reusable) > 0 {
		// 从可重用列表末尾取出一个标记
		flag = f.reusable[len(f.reusable)-1]
		f.reusable = f.reusable[:len(f.reusable)-1]
	} else {
		// 如果没有可重用的标记，使用新的标记值
		flag = f.nextFlag
		f.nextFlag++
	}

	// 标记这个标记正在等待
	f.waiting[flag] = true
	return flag, true
}

// WaitEvent 等待指定标记的事件
func (f *flagImpl) WaitEvent(flag FlagValue) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	// 检查标记是否在等待集合中
	if !f.waiting[flag] {
		return false
	}
	// 等待直到标记被释放
	for !f.released[flag] {
		f.cond.Wait()
	}
	// 标记已释放，从等待集合中移除
	delete(f.waiting, flag)
	delete(f.released, flag)
	// 将标记添加到可重用列表
	f.reusable = append(f.reusable, flag)
	return true
}

// ReleaseEvent 释放指定标记的事件
func (f *flagImpl) ReleaseEvent(flag FlagValue) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	// 检查标记是否在等待集合中
	if !f.waiting[flag] {
		return false
	}
	// 标记为已释放
	f.released[flag] = true
	// 通知所有等待的 goroutine
	f.cond.Broadcast()
	return true
}

// GetStatus 获取指定标记的状态
func (f *flagImpl) GetStatus(flag FlagValue) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 如果标记在等待集合中，表示有等待内容
	return f.waiting[flag]
}
