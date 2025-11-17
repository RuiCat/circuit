# circuit
Go实现的电路仿真,参考开源 [circuitjs](https://github.com/sharpie7/circuitjs1) 库的Go版本实现.
## 当前Bug列表

## 开发日志
  * [2025-11-17] 准备重写
    > 重构想法通过创建全局上下文来管理整个仿真过程.  
    > 通过函数工程模式对上下文进行加工同时让上下文保存整个仿真的数据与状态,通过事件来注册与获取指定的接口.
    * utils 实现基础底层结构包  
      1. Bitmap 位图标记结构
      2. Event 事件管理
      3. Flag 事件标记管理
    * maths 矩阵实现