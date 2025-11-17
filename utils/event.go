package utils

// EventType 注册的事件类型标记
type EventType uint16

// EventValue 事件值
type EventValue interface {
	Get() (value any) // 获取值
	Set(value any)    // 设置值
	Type() EventType  // 事件类型
}

// Event 事件接口
type Event interface {
	Type() EventType                  // 事件类型
	Callback() func(event EventValue) // 处理流程
	NewEvent() EventValue             // 创建事件
}

// Context 管理上下文
// @ 事件管理使用方法,先注册事件到上下文.然后在携程中获取事件,如果事件不需要处理则调用事件绑定的回调让事件注册结构体自行处理事件.
// @ 上下文处理过程中如果需要发送事件则通过上下文的发送事件接口函数将值发送出去.
type Context interface {
	EventReceive() <-chan Event                          // 获取事件
	EventSend(eventType EventType, eventValue any) bool  // 发送事件
	EventRegister(eventType EventType, event Event) bool // 注册事件
}

// contextImpl 实现 Context 接口
type contextImpl struct {
	events       map[EventType]Event // 注册的事件处理器
	eventChannel chan Event          // 事件接收通道
}

// NewContext 创建新的上下文实例
func NewContext() Context {
	return &contextImpl{
		events:       make(map[EventType]Event),
		eventChannel: make(chan Event, 100), // 缓冲通道，避免阻塞
	}
}

// EventReceive 获取事件接收通道
func (c *contextImpl) EventReceive() <-chan Event {
	return c.eventChannel
}

// EventSend 发送事件
func (c *contextImpl) EventSend(eventType EventType, eventValue any) bool {
	// 查找对应的事件处理器
	event, exists := c.events[eventType]
	if !exists {
		return false
	}
	// 创建新的事件值
	eventVal := event.NewEvent()
	if eventVal == nil {
		return false
	}
	// 设置事件值
	eventVal.Set(eventValue)
	c.eventChannel <- event
	return true
}

// EventRegister 注册事件
func (c *contextImpl) EventRegister(eventType EventType, event Event) bool {
	if _, ok := c.events[eventType]; ok {
		return false
	}
	c.events[eventType] = event
	return true
}
