package types

// EventType 定义事件类型
type EventType int

const (
	// Message 相关事件
	// client侧
	EventSendRequest                EventType = iota // 发送请求(CLIENT侧初始动作)
	EventReceiveProvisionalResponse                  // 接收临时响应(1xx)(CLIENT侧)
	EventReceive2xxResponse                          // 接收2xx响应(CLIENT侧)
	EventReceiveNon2xxFinalResponse                  // 接收非2xx最终响应(3xx/4xx/5xx/6xx)(CLIENT侧)
	// server侧
	EventReceiveRequest          // 接收请求(SERVER初始动作)
	EventSendProvisionalResponse // 发送临时响应(1xx)(SERVER侧)
	EventSend2xxResponse         // 发送2xx响应(SERVER侧)
	EventSendNon2xxFinalResponse // 发送非2xx最终响应(3xx/4xx/5xx/6xx)(SERVER侧)
	// 公共事件
	EventSendBYERequest    // 发送BYE请求
	EventReceiveBYERequest // 接收BYE请求
	EventTerminate         // 强制终止

	// Dialog 相关事件
	EventDialogCreated EventType = iota // Dialog 创建

	EventDialogStateChanged // Dialog 状态变化
	EventDialogTerminated   // Dialog 终止/移除

	// Transaction 相关事件
	EventTransactionCreated      // Transaction 创建
	EventTransactionStateChanged // Transaction 状态变化
	EventTransactionTerminated   // Transaction 终止/移除
	EventTransactionTimeout      // Transaction 超时
)

type SipEvent interface {
	// Type 返回事件类型
	Type() EventType
	// String 返回事件的字符串表示
	String() string
	// Context 返回事件的上下文信息
	context() map[string]interface{}
}
