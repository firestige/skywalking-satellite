package sip

import "github.com/apache/skywalking-satellite/plugins/receiver/sip/types"

type SessionManager struct {
	sessions map[string]*session
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*session),
	}
}

func (sm *SessionManager) GetOrCreateSession(msg types.SipMessage) (*session, error) {
	sessionID := msg.CallID() + "|" + msg.FromTag() + "|" + msg.ToTag()
	if existingSession, exists := sm.sessions[sessionID]; exists {
		return existingSession, nil
	}

	newSession := NewSession(sessionID)
	sm.sessions[sessionID] = newSession
	return newSession, nil
}

func (sm *SessionManager) DoOnRemoveSession(callId string, fn func(session *session)) {
	if session, exists := sm.sessions[callId]; exists {
		fn(session)
		delete(sm.sessions, callId)
	}
}
