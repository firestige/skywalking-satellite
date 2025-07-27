package sip

import (
	"fmt"

	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

type UserAgentClient struct {
	manager *SessionManager
}

func NewUserAgentClient(manager *SessionManager) *UserAgentClient {
	return &UserAgentClient{
		manager: manager,
	}
}

func (uac *UserAgentClient) SendMsg(msg types.SipMessage) error {
	// 特殊情况：in-dialog消息UAC可能发出响应
	if resp, ok := msg.(types.SipResponse); ok {
		session, err := uac.manager.GetOrCreateSession(resp)
		if err != nil {
			return err
		}
		dialog, err := session.GetOrCreateDialogIfAbsent(resp)
		if err != nil {
			return err
		}
		// 此时UAC响应UAS的请求，此时要分情况，如果是Info，则仅改变当前事务状态，如果是Bye或者Cancel，则需要结束Dialog，连带的Session也需要结束
		tx, err := dialog.GetOrCreateTransactionIfAbsent(resp)
		if err != nil {
			return err
		}
		if resp.Is1XX() {
			tx.ChangeState(types.NonInviteTransactionStateProceeding, types.NonInviteTransactionStateTrying)
		} else if resp.Is2XX() {
			tx.ChangeState(tx.State(), types.NonInviteTransactionStateTerminated)
	if msg.IsResponse() {
		if uac.session == nil {
			return nil // 如果没有会话，直接忽略响应
		}
		if uac.session.UAType() != types.UAClient {
			return fmt.Errorf("user agent type is not client: %v", uac.session.UAType())
		}
	}

	// non-in-dialog消息UAC只能发出请求
	return uac.session.handleUACMessage(msg)
}

func (uac *UserAgentClient) ReceiveMsg(msg types.SipMessage) error {
	if uac.session == nil {
		return fmt.Errorf("session is nil")
	}
	if uac.session.UAType() != types.UAClient {
		return fmt.Errorf("user agent type is not client: %v", uac.session.UAType())
	}
	return uac.session.handleUACMessage(msg)
}
