package sip

type sipMessage struct {
	callId  string
	cSeq    string
	headers map[string]string
	body    string
	request bool
}

type sipRequest struct {
	sipMessage
	method      string
	requestLine string
}

type sipResponse struct {
	sipMessage
	status     int
	statusLine string
}

// SipMessage interface implementations
func (m *sipMessage) CallId() string {
	return m.callId
}

func (m *sipMessage) CSeq() string {
	return m.cSeq
}

func (m *sipMessage) Headers() map[string]string {
	return m.headers
}

func (m *sipMessage) Body() string {
	return m.body
}

func (m *sipMessage) IsRquest() bool {
	return m.request
}

// SipRequest interface implementations
func (r *sipRequest) Method() string {
	return r.method
}

func (r *sipRequest) RequestLine() string {
	return r.requestLine
}

// SipResponse interface implementations
func (r *sipResponse) Status() int {
	return r.status
}

func (r *sipResponse) StatusLine() string {
	return r.statusLine
}
