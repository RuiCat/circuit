package can

import "circuit/gpio/driver"

// CANProtocolSLCAN 返回SLCAN (ASCII) 协议的配置
// SLCAN是常见的UART转CAN协议，使用ASCII字符命令
func CANProtocolSLCAN() *driver.CANProtocolConfig {
	return &driver.CANProtocolConfig{
		// SLCAN使用ASCII命令，需要转换为字节
		InitCmd:             []byte("O\r"),              // 打开CAN总线
		SetFilterCmd:        []byte("M{ID}{MASK}"),      // 设置过滤器 (ID和掩码为8位十六进制)
		GetStatusCmd:        []byte("F\r"),              // 获取状态标志
		SetModeCmd:          []byte(""),                 // SLCAN不支持模式切换
		ClearErrorsCmd:      []byte(""),                 // SLCAN不支持清除错误
		GetErrorCountersCmd: []byte(""),                 // SLCAN不支持错误计数器
		SetBaudRateCmd:      []byte("S{BAUDRATE}"),      // 设置波特率 (0-8对应不同波特率)
		SendFrameCmd:        []byte("t{ID}{DLC}{DATA}"), // 发送标准帧 (11位ID)
		// 响应解析配置
		StatusRespLength:        1, // "F"命令返回单个字符
		ErrorCountersRespLength: 0, // 不支持
		// 帧解析配置
		FrameStartMarker: []byte("\r"), // SLCAN帧以回车开始
		FrameMinLength:   5,            // 最小帧长度 (如 "t1230")
	}
}

// CANProtocolBinary 返回常见的二进制协议配置
// 许多UART转CAN适配器使用二进制协议，通常以0xAA 0x55开始
// 注意：此示例使用ASCII占位符字符串，实际二进制协议可能需要不同的处理方式
func CANProtocolBinary() *driver.CANProtocolConfig {
	// 构建发送帧命令模板：0xAA 0x55 0x09 [4字节ID] [1字节属性] [1字节DLC] [最多8字节数据]
	// 使用ASCII占位符以便替换，实际使用时会替换为二进制数据
	sendFrameTemplate := []byte{0xAA, 0x55, 0x09}
	sendFrameTemplate = append(sendFrameTemplate, []byte("{ID}")...)   // 4字节ID占位符
	sendFrameTemplate = append(sendFrameTemplate, []byte("{ATTR}")...) // 1字节属性占位符
	sendFrameTemplate = append(sendFrameTemplate, []byte("{DLC}")...)  // 1字节DLC占位符
	sendFrameTemplate = append(sendFrameTemplate, []byte("{DATA}")...) // 数据占位符
	return &driver.CANProtocolConfig{
		// 二进制协议命令（使用ASCII占位符字符串以便替换）
		InitCmd:             []byte{0xAA, 0x55, 0x01},
		SetFilterCmd:        append([]byte{0xAA, 0x55, 0x02}, []byte("{ID}{MASK}")...), // 8字节占位符
		GetStatusCmd:        []byte{0xAA, 0x55, 0x03},
		SetModeCmd:          append([]byte{0xAA, 0x55, 0x04}, []byte("{MODE}")...),
		ClearErrorsCmd:      []byte{0xAA, 0x55, 0x05},
		GetErrorCountersCmd: []byte{0xAA, 0x55, 0x06},
		SetBaudRateCmd:      append([]byte{0xAA, 0x55, 0x07}, []byte("{BAUDRATE}")...), // 4字节占位符
		SendFrameCmd:        sendFrameTemplate,
		// 响应解析配置
		StatusRespLength:        4, // 32位状态值
		ErrorCountersRespLength: 8, // 两个32位计数器
		// 帧解析配置
		FrameStartMarker: []byte{0xAA, 0x55, 0x0A}, // 接收帧起始标记
		FrameMinLength:   10,                       // 最小帧长度 (ID+属性+数据)
	}
}

// CANProtocolCANable 返回CANable适配器的配置
// CANable是基于STM32的开源USB转CAN适配器，通常使用SLCAN协议
func CANProtocolCANable() *driver.CANProtocolConfig {
	// CANable通常使用SLCAN协议，但也支持二进制模式
	// 这里提供一个配置示例
	return &driver.CANProtocolConfig{
		InitCmd:             []byte("C\r"),              // 关闭CAN (CANable特定)
		SetFilterCmd:        []byte("M{ID}{MASK}"),      // 设置过滤器
		GetStatusCmd:        []byte("F\r"),              // 获取状态
		SetModeCmd:          []byte(""),                 // 标准模式/监听模式通过"O"/"L"命令
		ClearErrorsCmd:      []byte(""),                 // 不支持
		GetErrorCountersCmd: []byte(""),                 // 不支持
		SetBaudRateCmd:      []byte("S{BAUDRATE}"),      // 设置波特率
		SendFrameCmd:        []byte("t{ID}{DLC}{DATA}"), // 发送标准帧
		// 响应解析配置
		StatusRespLength:        1,
		ErrorCountersRespLength: 0,
		// 帧解析配置
		FrameStartMarker: []byte("\r"),
		FrameMinLength:   5,
	}
}

// ExampleCANConfig 返回一个示例CAN配置
func ExampleCANConfig() *driver.CANConfig {
	return &driver.CANConfig{
		Protocol:    CANProtocolBinary(), // 使用二进制协议作为示例
		Mode:        0,                   // 正常模式
		FilterMode:  1,                   // 单过滤器模式
		FilterID:    0x123,               // 示例过滤器ID
		FilterMask:  0x7FF,               // 标准帧掩码
		AutoRetrans: true,                // 自动重传
		TxPriority:  1,                   // 发送优先级
	}
}
