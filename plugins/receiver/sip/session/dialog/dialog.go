package dialog

import "github.com/apache/skywalking-satellite/plugins/receiver/sip/types"

// DialogEvent 定义所有可能的事件
type DialogEvent int

const (
	EventSendRequest                DialogEvent = iota // 发送请求（UAC侧初始动作）
	EventReceiveRequest                                // 接收请求（UAS侧初始动作）
	EventSendProvisionalResponse                       // 发送临时响应（1xx）（UAS侧）
	EventReceiveProvisionalResponse                    // 接收临时响应（1xx）（UAC侧）
	EventSend2xxResponse                               // 发送2xx响应
	EventReceive2xxResponse                            // 接收2xx响应
	EventSendNon2xxFinalResponse                       // 发送非2xx最终响应（3xx/4xx/5xx/6xx）
	EventReceiveNon2xxFinalResponse                    // 接收非2xx最终响应（3xx/4xx/5xx/6xx）
	EventSendBYERequest                                // 发送BYE请求
	EventReceiveBYERequest                             // 接收BYE请求
	EventTerminate                                     // 强制终止
)

func convertFrom(event *types.SipEvent) DialogEvent {
	switch event.Type {
	case types.EventSendRequest:
		return EventSendRequest
	case types.EventReceiveRequest:
		return EventReceiveRequest
	case types.EventSendProvisionalResponse:
		return EventSendProvisionalResponse
	case types.EventReceiveProvisionalResponse:
		return EventReceiveProvisionalResponse
	case types.EventSend2xxResponse:
		return EventSend2xxResponse
	case types.EventReceive2xxResponse:
		return EventReceive2xxResponse
	case types.EventSendNon2xxFinalResponse:
		return EventSendNon2xxFinalResponse
	case types.EventReceiveNon2xxFinalResponse:
		return EventReceiveNon2xxFinalResponse
	case types.EventSendBYERequest:
		return EventSendBYERequest
	case types.EventReceiveBYERequest:
		return EventReceiveBYERequest
	case types.EventTerminate:
		return EventTerminate
	default:
		return -1 // 未知事件
	}
}

type Dialog interface {
	types.Dialog
}
