package dialog

import (
	"fmt"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/utils"
)

type DialogState interface {
	HandleMessage(ctx *DialogContext, msg types.SipMessage) (DialogState, error)
	Enter(ctx *DialogContext)
	Exit(ctx *DialogContext)
}

type EarlyState struct{}

func (s *EarlyState) Enter(ctx *DialogContext) {
	// 初始化对话状态
}

func (s *EarlyState) HandleMessage(ctx *DialogContext, msg types.SipMessage) (DialogState, error) {
	if resp, ok := msg.(types.SipResponse); ok {
		if utils.IsProvisionalResponse(resp) {
			// 1xx响应，保持Early
			return s, nil
		}
		if utils.Is2XXResponse(resp) {
			// 2xx响应，进入Confirmed
			next := &ConfirmedState{}
			return next, nil
		}
		if utils.IsNon2XXFinalResponse(resp) {
			// 非2xx最终响应，进入Terminated
			next := &TerminatedState{}
			return next, nil
		}
		// 其他响应，忽略
		return s, nil
	}
	if req, ok := msg.(types.SipRequest); ok {
		switch req.Method() {
		case types.MethodBye, types.MethodCancel:
			// BYE或CANCEL请求，进入Terminated
			next := &TerminatedState{}
			return next, nil
		default:
			// 其他请求，保持Early
			log.Logger.WithError(fmt.Errorf("EarlyState: unhandled request method %s", req.Method())).Debugf("Received SIP request: %s", req.String())
			return s, nil
		}
	}
	return s, fmt.Errorf("EarlyState: unhandled message type %T", msg)
}

func (s *EarlyState) Exit(ctx *DialogContext) {
	// 清理对话状态
}

type ConfirmedState struct{}

func (s *ConfirmedState) Enter(ctx *DialogContext) {
	// 初始化已确认状态
}

func (s *ConfirmedState) HandleMessage(ctx *DialogContext, msg types.SipMessage) (DialogState, error) {
	if req, ok := msg.(types.SipRequest); ok {
		switch req.Method() {
		case types.MethodBye, types.MethodCancel:
			// BYE或CANCEL请求，进入Terminated
			next := &TerminatedState{}
			return next, nil
		default:
			// 其他请求，保持Confirmed
			return s, fmt.Errorf("ConfirmedState: unhandled request method %s", req.Method())
		}
	}
	return s, fmt.Errorf("ConfirmedState: unhandled message type %T", msg)
}

func (s *ConfirmedState) Exit(ctx *DialogContext) {
	// 清理已确认状态
}

type TerminatedState struct{}

func (s *TerminatedState) Enter(ctx *DialogContext) {
	// 初始化终止状态
}

func (s *TerminatedState) HandleMessage(ctx *DialogContext, msg types.SipMessage) (DialogState, error) {
	// 终止态不再处理任何事件
	return nil, fmt.Errorf("dialog already terminated")
}

func (s *TerminatedState) Exit(ctx *DialogContext) {
	// 清理终止状态
}
