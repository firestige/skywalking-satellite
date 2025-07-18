package sip

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
