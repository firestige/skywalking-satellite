package types

import (
	"sync"
	"time"

	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

type Connection struct {
	id       string
	SrcIp    string
	SrcPort  int
	DstIp    string
	DstPort  int
	Protocol string
}

type WithConnection interface {
	Connection() *Connection
}

type SipMessage interface {
	CallId() string
	CSeq() string
	Headers() map[string]string
	Body() string
	IsRquest() bool
}

type SipRequest interface {
	Method() string
	RequestLine() string
	SipMessage
}

type SipResponse interface {
	Status() int
	StatusLine() string
	SipMessage
}

type SipSessionManager interface {
	GetOrCreateSession(msg SipMessage) (*Session, error)
	DoOnRemoveSession(callId string, fn func(session *Session))
	Prepare() error
	Stop()
}

// 会话对象，主要通过callId和cseq来唯一标识一个SIP会话中的请求和响应
type Session struct {
	CallId       string // SIP Call ID
	CurrentCseq  string // 当前 CSeq，Cseq一般由数字和方法组成，如 "1 INVITE"
	CurrentSpan  int32  // 当前 span ID，表示当前需要处理的 span
	Segment      *agent.SegmentObject
	LastModified time.Time    // 会话最后修改时间
	mu           sync.RWMutex // 保护并发访问
}

func (s *Session) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return "CallId: " + s.CallId + ", CSeq: " + s.CurrentCseq + ", CurrentSpan: " + string(s.CurrentSpan) +
		", LastModified: " + s.LastModified.String() + ", Segment: " + s.Segment.String()
}
