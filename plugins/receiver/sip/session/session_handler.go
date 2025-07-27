package sip

import (
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

type SessionHandler struct {
	sessionManager *SessionManager
}

func NewSessionHandler(sessionManager *SessionManager) *SessionHandler {
	return &SessionHandler{
		sessionManager: sessionManager,
	}
}

func (sh *SessionHandler) Handle(msg types.SipMessage) error {
	panic("not implemented") // TODO: Implement session handling logic
}
