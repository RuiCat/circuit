package utils

// EventType 注册的事件类型标记
type EventType uint16

// EventMark 用于区分事件的标记
type EventMark uint8

// EventValue 事件值
type EventValue interface {
	Get() (value any)       // 获取值
	Set(value any)          // 设置值
	SetMark(mark EventMark) // 设置标记
	Mark() EventMark        // 标记类型
	Type() EventType        // 事件类型
}

// Event 事件接口
type Event interface {
	Type() EventType           // 事件类型
	Callback(event EventValue) // 处理流程
	EventValue() EventValue    // 事件内容
}

// Context 管理上下文
// @ 事件管理使用方法,先注册事件到上下文.然后在携程中获取事件,如果事件不需要处理则调用事件绑定的回调让事件注册结构体自行处理事件.
// @ 上下文处理过程中如果需要发送事件则通过上下文的发送事件接口函数将值发送出去.
type Context interface {
	Close()                                          // 关闭通道
	GetEvent(eventType EventType) Event              // 得到事件
	GetValue(eventType EventType) (EventValue, bool) // 事件内容
	EventReceive() <-chan EventValue                 // 获取事件
	EventSend(eventType EventType, val any) bool     // 发送事件
	EventSendValue(value EventValue) bool            // 发送事件
	EventRegister(event Event) bool                  // 注册事件
	Callback(value EventValue)                       // 调用默认处理
}

// contextImpl 实现 Context 接口
type contextImpl struct {
	events       map[EventType]Event // 注册的事件处理器
	eventChannel chan EventValue     // 事件接收通道
}

// NewContext 创建新的上下文实例
func NewContext() Context {
	return &contextImpl{
		events:       make(map[EventType]Event),
		eventChannel: make(chan EventValue, 100), // 缓冲通道，避免阻塞
	}
}

// GetEvent 得到事件
func (c *contextImpl) GetEvent(eventType EventType) Event {
	return c.events[eventType]
}

// GetValue 事件内容
func (c *contextImpl) GetValue(eventType EventType) (EventValue, bool) {
	if v, ok := c.events[eventType]; ok {
		return v.EventValue(), true
	}
	return nil, false
}

// EventReceive 获取事件接收通道
func (c *contextImpl) EventReceive() <-chan EventValue {
	return c.eventChannel
}

// EventSend 发送事件
func (c *contextImpl) EventSend(eventType EventType, val any) bool {
	if eve, ok := c.events[eventType]; ok {
		value := eve.EventValue()
		value.Set(val)
		c.eventChannel <- value
		return true
	}
	return false
}

// EventSendValue 发送事件值
func (c *contextImpl) EventSendValue(value EventValue) bool {
	if _, ok := c.events[value.Type()]; ok {
		c.eventChannel <- value
		return true
	}
	return false
}

// EventRegister 注册事件
func (c *contextImpl) EventRegister(event Event) bool {
	eventType := event.Type()
	if _, ok := c.events[eventType]; ok {
		return false
	}
	c.events[eventType] = event
	return true
}

// Callback 调用默认处理
func (c *contextImpl) Callback(value EventValue) {
	c.GetEvent(value.Type()).Callback(value)
}

// Close 关闭通道
func (c *contextImpl) Close() {
	close(c.eventChannel)
}
