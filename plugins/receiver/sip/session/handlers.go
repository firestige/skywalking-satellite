package session

import (
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

// InviteSessionHandler INVITE会话处理器
type InviteSessionHandler struct{}

func (h *InviteSessionHandler) HandleRequest(dialog *Dialog, transaction *Transaction, msg types.SipRequest) error {
	switch msg.Method() {
	case "INVITE":
		return h.handleInviteRequest(dialog, transaction, msg)
	case "ACK":
		return h.handleAckRequest(dialog, transaction, msg)
	case "BYE":
		return h.handleByeRequest(dialog, transaction, msg)
	case "CANCEL":
		return h.handleCancelRequest(dialog, transaction, msg)
	}
	return nil
}

func (h *InviteSessionHandler) HandleResponse(dialog *Dialog, transaction *Transaction, msg types.SipResponse) error {
	// 处理INVITE会话的响应
	return nil
}

func (h *InviteSessionHandler) GetSessionType() SessionType {
	return SessionTypeInvite
}

func (h *InviteSessionHandler) handleInviteRequest(dialog *Dialog, transaction *Transaction, msg types.SipRequest) error {
	// 处理INVITE请求逻辑
	transaction.State = TransactionStateProceeding
	return nil
}

func (h *InviteSessionHandler) handleAckRequest(dialog *Dialog, transaction *Transaction, msg types.SipRequest) error {
	// 处理ACK请求逻辑
	transaction.State = TransactionStateConfirmed
	return nil
}

func (h *InviteSessionHandler) handleByeRequest(dialog *Dialog, transaction *Transaction, msg types.SipRequest) error {
	// 处理BYE请求逻辑
	dialog.State = DialogStateTerminated
	transaction.State = TransactionStateCompleted
	return nil
}

func (h *InviteSessionHandler) handleCancelRequest(dialog *Dialog, transaction *Transaction, msg types.SipRequest) error {
	// 处理CANCEL请求逻辑
	transaction.State = TransactionStateCompleted
	return nil
}

// RegisterSessionHandler REGISTER会话处理器
type RegisterSessionHandler struct{}

func (h *RegisterSessionHandler) HandleRequest(dialog *Dialog, transaction *Transaction, msg types.SipRequest) error {
	switch msg.Method() {
	case "REGISTER":
		return h.handleRegisterRequest(dialog, transaction, msg)
	}
	return nil
}

func (h *RegisterSessionHandler) HandleResponse(dialog *Dialog, transaction *Transaction, msg types.SipResponse) error {
	// 处理REGISTER会话的响应
	return nil
}

func (h *RegisterSessionHandler) GetSessionType() SessionType {
	return SessionTypeRegister
}

func (h *RegisterSessionHandler) handleRegisterRequest(dialog *Dialog, transaction *Transaction, msg types.SipRequest) error {
	// 处理REGISTER请求逻辑
	transaction.State = TransactionStateProceeding
	// REGISTER通常是无状态的，会话生命周期较短
	dialog.State = DialogStateConfirmed
	return nil
}

// SubscribeSessionHandler SUBSCRIBE会话处理器（扩展示例）
type SubscribeSessionHandler struct{}

func (h *SubscribeSessionHandler) HandleRequest(dialog *Dialog, transaction *Transaction, msg types.SipRequest) error {
	switch msg.Method() {
	case "SUBSCRIBE":
		return h.handleSubscribeRequest(dialog, transaction, msg)
	case "NOTIFY":
		return h.handleNotifyRequest(dialog, transaction, msg)
	}
	return nil
}

func (h *SubscribeSessionHandler) HandleResponse(dialog *Dialog, transaction *Transaction, msg types.SipResponse) error {
	// 处理SUBSCRIBE会话的响应
	return nil
}

func (h *SubscribeSessionHandler) GetSessionType() SessionType {
	return SessionTypeSubscribe
}

func (h *SubscribeSessionHandler) handleSubscribeRequest(dialog *Dialog, transaction *Transaction, msg types.SipRequest) error {
	// 处理SUBSCRIBE请求逻辑
	transaction.State = TransactionStateProceeding
	return nil
}

func (h *SubscribeSessionHandler) handleNotifyRequest(dialog *Dialog, transaction *Transaction, msg types.SipRequest) error {
	// 处理NOTIFY请求逻辑
	transaction.State = TransactionStateProceeding
	return nil
}
