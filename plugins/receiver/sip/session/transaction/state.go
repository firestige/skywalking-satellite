package transaction

import (
	"fmt"
)

type TransactionState interface {
	HandleEvent(ctx *TransactionContext, event TransactionEvent) (TransactionState, error)
	Enter(ctx *TransactionContext)
	Exit(ctx *TransactionContext)
}

// ---- Non-INVITE Transaction States ----

type NonInviteTryingState struct{}

func (s *NonInviteTryingState) Enter(ctx *TransactionContext) {
	ctx.tx.StartTimer(TimerE, T1)
}

func (s *NonInviteTryingState) HandleEvent(ctx *TransactionContext, event TransactionEvent) (TransactionState, error) {
	switch event {
	case EventSendRequest, EventReceiveRequest:
		// 保持Trying
		return s, nil
	case EventReceive1xx, EventSend1xx:
		return &NonInviteProceedingState{}, nil
	case EventReceiveFinal, EventSendFinal:
		return &NonInviteCompletedState{}, nil
	default:
		return nil, fmt.Errorf("unexpected event %v in NonInviteTryingState", event)
	}
}

func (s *NonInviteTryingState) Exit(ctx *TransactionContext) {

}

type NonInviteProceedingState struct{}

func (s *NonInviteProceedingState) Enter(ctx *TransactionContext) {
	ctx.tx.StartTimer(TimerF, 64*T1)
}

func (s *NonInviteProceedingState) HandleEvent(ctx *TransactionContext, event TransactionEvent) (TransactionState, error) {
	switch event {
	case EventReceive1xx, EventSend1xx:
		// 保持Proceeding
		return s, nil
	case EventReceiveFinal, EventSendFinal:
		return &NonInviteCompletedState{}, nil
	case EventTimerJ:
		return &NonInviteTerminatedState{}, nil
	default:
		return nil, fmt.Errorf("unexpected event %v in NonInviteProceedingState", event)
	}
}

func (s *NonInviteProceedingState) Exit(ctx *TransactionContext) {

}

type NonInviteCompletedState struct{}

func (s *NonInviteCompletedState) Enter(ctx *TransactionContext) {
	// 通常在UAS侧启动TimerJ，UAC侧启动TimerK
	// 这里假设都启动TimerJ，具体可根据角色区分
	ctx.tx.StartTimer(TimerJ, 64*T1)
}

func (s *NonInviteCompletedState) HandleEvent(ctx *TransactionContext, event TransactionEvent) (TransactionState, error) {
	switch event {
	case EventTimerJ, EventTimerK:
		return &NonInviteTerminatedState{}, nil
	default:
		return nil, fmt.Errorf("unexpected event %v in NonInviteCompletedState", event)
	}
}

func (s *NonInviteCompletedState) Exit(ctx *TransactionContext) {
	ctx.tx.CancelTimer(TimerF)
	ctx.tx.CancelTimer(TimerK)
}

type NonInviteTerminatedState struct{}

func (s *NonInviteTerminatedState) Enter(ctx *TransactionContext) {
	// 事务终止，无需操作
}

func (s *NonInviteTerminatedState) HandleEvent(ctx *TransactionContext, event TransactionEvent) (TransactionState, error) {
	return nil, fmt.Errorf("transaction already terminated")
}

func (s *NonInviteTerminatedState) Exit(ctx *TransactionContext) {}

// ---- INVITE Transaction States ----

type InviteCallingState struct{}

func (s *InviteCallingState) Enter(ctx *TransactionContext) {
	ctx.tx.StartTimer(TimerA, T1)
	ctx.tx.StartTimer(TimerB, 64*T1)
}

func (s *InviteCallingState) HandleEvent(ctx *TransactionContext, event TransactionEvent) (TransactionState, error) {
	switch event {
	case EventSendRequest:
		return s, nil
	case EventReceive1xx, EventSend1xx:
		return &InviteProceedingState{}, nil
	case EventReceiveFinal, EventSendFinal:
		return &InviteCompletedState{}, nil
	case EventReceive2xx:
		return &InviteTerminatedState{}, nil
	default:
		return nil, fmt.Errorf("unexpected event %v in InviteCallingState", event)
	}
}

func (s *InviteCallingState) Exit(ctx *TransactionContext) {
	ctx.tx.CancelTimer(TimerA)
	ctx.tx.CancelTimer(TimerB)
}

type InviteProceedingState struct{}

func (s *InviteProceedingState) Enter(ctx *TransactionContext) {
	ctx.tx.StartTimer(TimerC, 64*T1)
}

func (s *InviteProceedingState) HandleEvent(ctx *TransactionContext, event TransactionEvent) (TransactionState, error) {
	switch event {
	case EventReceive1xx, EventSend1xx:
		return s, nil
	case EventReceiveFinal, EventSendFinal:
		return &InviteCompletedState{}, nil
	case EventReceive2xx:
		return &InviteTerminatedState{}, nil
	default:
		return nil, fmt.Errorf("unexpected event %v in InviteProceedingState", event)
	}
}

func (s *InviteProceedingState) Exit(ctx *TransactionContext) {
	ctx.tx.CancelTimer(TimerC)
}

type InviteCompletedState struct{}

func (s *InviteCompletedState) Enter(ctx *TransactionContext) {
	ctx.tx.StartTimer(TimerD, 64*T1)
}

func (s *InviteCompletedState) HandleEvent(ctx *TransactionContext, event TransactionEvent) (TransactionState, error) {
	switch event {
	case EventSendACK, EventReceiveACK:
		return &InviteConfirmedState{}, nil
	case EventTimerI:
		return &InviteTerminatedState{}, nil
	default:
		return nil, fmt.Errorf("unexpected event %v in InviteCompletedState", event)
	}
}

func (s *InviteCompletedState) Exit(ctx *TransactionContext) {
	ctx.tx.CancelTimer(TimerD)
}

type InviteConfirmedState struct{}

func (s *InviteConfirmedState) Enter(ctx *TransactionContext) {
	ctx.tx.StartTimer(TimerI, T1)
}

func (s *InviteConfirmedState) HandleEvent(ctx *TransactionContext, event TransactionEvent) (TransactionState, error) {
	switch event {
	case EventTimerI:
		return &InviteTerminatedState{}, nil
	default:
		return nil, fmt.Errorf("unexpected event %v in InviteConfirmedState", event)
	}
}

func (s *InviteConfirmedState) Exit(ctx *TransactionContext) {
	ctx.tx.CancelTimer(TimerI)
}

type InviteTerminatedState struct{}

func (s *InviteTerminatedState) Enter(ctx *TransactionContext) {
	// 事务终止，无需操作
}

func (s *InviteTerminatedState) HandleEvent(ctx *TransactionContext, event TransactionEvent) (TransactionState, error) {
	return nil, fmt.Errorf("transaction already terminated")
}

func (s *InviteTerminatedState) Exit(ctx *TransactionContext) {}
