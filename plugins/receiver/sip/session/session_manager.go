package sip

import (
	"fmt"

	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

type SessionManager struct {
	sessions       map[string]*session
	eventChan      chan *types.SessionEvent // 用于处理会话事件
	eventListeners []func(event *types.SessionEvent)
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:  make(map[string]*session),
		eventChan: make(chan *types.SessionEvent, 100), // 创建事件通道
	}
}

func (sm *SessionManager) GetOrCreateSession(msg types.SipMessage) (*session, error) {
	// 使用 Call-ID 构建会话标志
	sessionId, exist := msg.Headers()[string(types.HeaderNameX_ICC_CALL_ID)]
	if !exist {
		sessionId = msg.CallID()
	}
	session, exist := sm.sessions[sessionId]
	if exist {
		return session, nil
	}
	// 如果不存在，则创建新的会话
	ua := getUAType(msg)
	if ua == types.UAUnknown {
		return nil, fmt.Errorf("in-dialog message should not create session: %v", msg)
	}
	newSession := NewSession(sessionId, getSessionType(msg), ua, sm.eventChan)
	sm.sessions[sessionId] = newSession
	return newSession, nil
}

func (sm *SessionManager) DoOnRemoveSession(callId string, fn func(session *session)) {
	if session, exists := sm.sessions[callId]; exists {
		fn(session)
		delete(sm.sessions, callId)
	}
}

func getSessionType(msg types.SipMessage) types.SessionType {
	method := extractMethodFromCseq(msg.CSeq())
	switch method {
	case types.Invite:
		return types.SessionTypeInvite
	case types.Subscribe:
		return types.SessionTypeSubscribe
	case types.Notify:
		return types.SessionTypeNotify
	default:
		return types.SessionTypeUnknown
	}
}

func getUAType(msg types.SipMessage) types.UAType {
	// 利用msg的Direction和请求与响应来判断UA类型
	// 对UAS而言inbound收到来自外部的请求，outbound发送到外部的响应
	// 对UAC而言inbound收到来自外部的响应，outbound发送到外部的请求
	// 特殊的情况是in-dialog消息,主要是Info/Bye/CANCEL,无论UAC还是UAS都可以发起,但此时不应该创建会话
	if msg.Direction() == types.DirectionInbound {
		if msg.IsRequest() {
			return types.UAServer
		}
		return types.UAClient
	}
	if msg.Direction() == types.DirectionOutbound {
		if msg.IsRequest() {
			return types.UAClient
		}
		return types.UAServer
	}
	return types.UAUnknown
}
