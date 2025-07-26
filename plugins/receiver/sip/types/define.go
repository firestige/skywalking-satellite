package types

import (
	"sync"

	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

type Connection struct {
	id        string
	SrcIp     string
	SrcPort   int
	DstIp     string
	DstPort   int
	Protocol  string
	Direction Direction
}

type WithConnection interface {
	Connection() *Connection
}

type SipSessionManager interface {
	GetOrCreateSession(msg SipMessage) (*Session, error)
	DoOnRemoveSession(callId string, fn func(session *Session))
	Prepare() error
	Stop()
}

// 会话对象，主要通过callId和cseq来唯一标识一个SIP会话中的请求和响应
type Session struct {
	ID      string // SIP Call ID
	CallID  string // SIP Call ID
	State   string // 会话状态
	Type    string // 会话类型
	Segment *agent.SegmentObject
	mu      sync.RWMutex // 保护并发访问
}

func (s *Session) String() string {
	return "Session{" +
		"Id='" + s.ID + "'" +
		", CallId='" + s.CallID + "'" +
		", State='" + s.State + "'" +
		", Type='" + s.Type + "'" +
		"}"
}
