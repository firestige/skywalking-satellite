package sip

import (
	"strings"

	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	"github.com/ghettovoice/gosip/sip"
)

type sipMessage struct {
	delegate   sip.Message // 使用 sipgo 的 Message 接口
	headers    map[string]string
	connection *types.Connection // 添加连接信息
}

type goSipRequest struct {
	sipMessage
	method types.Method
}

type goSipResponse struct {
	sipMessage
	status types.StatusCode
}

func FromGoSip(msg sip.Message, conn *types.Connection) types.SipMessage {
	headers := make(map[string]string)
	for _, v := range msg.Headers() {
		headers[v.Name()] = v.Value()
	}
	if req, ok := msg.(sip.Request); ok {
		return &goSipRequest{
			sipMessage: sipMessage{
				delegate:   msg,
				headers:    headers,
				connection: conn,
			},
			method: types.Method(req.Method()),
		}
	}
	if res, ok := msg.(sip.Response); ok {
		return &goSipResponse{
			sipMessage: sipMessage{
				delegate:   msg,
				headers:    headers,
				connection: conn,
			},
			status: types.StatusCode(res.StatusCode()),
		}
	}
	return nil
}

// SipMessage interface implementations
func (m *sipMessage) CallId() string {
	id, _ := m.delegate.CallID()
	return id.Value()
}

func (m *sipMessage) CSeq() string {
	cseq, _ := m.delegate.CSeq()
	return cseq.Value()
}

func (m *sipMessage) Headers() map[string]string {
	headers := make(map[string]string)
	for _, v := range m.delegate.Headers() {
		headers[v.Name()] = v.Value()
	}
	return headers
}

func (m *sipMessage) Body() string {
	return m.delegate.Body()
}

func (m *sipMessage) IsRequest() bool {
	return isRequest(m.delegate.StartLine())
}

func (m *sipMessage) Connection() *types.Connection {
	return m.connection
}

func (m *sipMessage) String() string {
	return m.delegate.String()
}

// SipRequest interface implementations
func (r *goSipRequest) Method() string {
	return string(r.method)
}

func (r *goSipRequest) RequestLine() string {
	return r.delegate.StartLine()
}

// SipResponse interface implementations
func (r *goSipResponse) Status() int {
	return int(r.status)
}

func (r *goSipResponse) StatusLine() string {
	return r.delegate.StartLine()
}

func isRequest(startLine string) bool {
	// SIP request lines contain precisely two spaces.
	if strings.Count(startLine, " ") != 2 {
		return false
	}

	// Check that the version string starts with SIP.
	parts := strings.Split(startLine, " ")
	if len(parts) < 3 {
		return false
	} else if len(parts[2]) < 3 {
		return false
	} else {
		return strings.ToUpper(parts[2][:3]) == "SIP"
	}
}
