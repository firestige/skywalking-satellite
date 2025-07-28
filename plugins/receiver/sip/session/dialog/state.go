package dialog

import "fmt"

type DialogState interface {
	HandleEvent(ctx *DialogContext, event DialogEvent) (DialogState, error)
	Enter(ctx *DialogContext)
	Exit(ctx *DialogContext)
}

type EarlySate struct{}

func (s *EarlySate) Enter(ctx *DialogContext) {
	// 初始化对话状态
}

func (s *EarlySate) HandleEvent(ctx *DialogContext, event DialogEvent) (DialogState, error) {
	switch event {
	case EventSendProvisionalResponse, EventReceiveProvisionalResponse:
		// 1xx响应，保持Early
		return s, nil
	case EventSend2xxResponse, EventReceive2xxResponse:
		// 2xx响应，进入Confirmed
		next := &ConfirmedState{}
		s.Exit(ctx)
		next.Enter(ctx)
		return next, nil
	case EventSendNon2xxFinalResponse, EventReceiveNon2xxFinalResponse:
		// 非2xx最终响应，进入Terminated
		next := &TerminatedState{}
		s.Exit(ctx)
		next.Enter(ctx)
		return next, nil
	case EventSendBYERequest, EventReceiveBYERequest, EventTerminate:
		// BYE或强制终止，直接进入Terminated
		next := &TerminatedState{}
		s.Exit(ctx)
		next.Enter(ctx)
		return next, nil
	default:
		return s, fmt.Errorf("EarlyState: unhandled event %v", event)
	}
}

func (s *EarlySate) Exit(ctx *DialogContext) {
	// 清理对话状态
}

type ConfirmedState struct{}

func (s *ConfirmedState) Enter(ctx *DialogContext) {
	// 初始化已确认状态
}

func (s *ConfirmedState) HandleEvent(ctx *DialogContext, event DialogEvent) (DialogState, error) {
	switch event {
	case EventSendBYERequest, EventReceiveBYERequest, EventTerminate:
		// BYE或强制终止，进入Terminated
		next := &TerminatedState{}
		s.Exit(ctx)
		next.Enter(ctx)
		return next, nil
	default:
		return s, fmt.Errorf("ConfirmedState: unhandled event %v", event)
	}
}

func (s *ConfirmedState) Exit(ctx *DialogContext) {
	// 清理已确认状态
}

type TerminatedState struct{}

func (s *TerminatedState) Enter(ctx *DialogContext) {
	// 初始化终止状态
}

func (s *TerminatedState) HandleEvent(ctx *DialogContext, event DialogEvent) (DialogState, error) {
	// 终止态不再处理任何事件
	return nil, fmt.Errorf("dialog already terminated")
}

func (s *TerminatedState) Exit(ctx *DialogContext) {
	// 清理终止状态
}
