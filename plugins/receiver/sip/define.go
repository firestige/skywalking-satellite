package sip

type SipMessage interface {
	CallId() string
	CSeq() string
	Headers() map[string]string
	Body() []byte
}

type SipRequest interface {
	RequestLine() string
	SipMessage
}

type SipResponse interface {
	StatusLine() string
	SipMessage
}
