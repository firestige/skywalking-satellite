package packet

import (
	"bytes"
	"strings"
)

type LengthFieldBaseDecoder struct {
	// Length field size in bytes
}

// HTTPLikeParser 类HTTP报文解析器
type HTTPLikeParser struct {
	// 零拷贝缓冲区
	requestBuffer  []byte
	responseBuffer []byte

	// 状态
	state      ParseState
	contentLen int
	chunked    bool

	// 结果通道
	MessageChan chan *HTTPLikeMessage
}

type ParseState int

const (
	StateHeaders ParseState = iota
	StateBody
	StateComplete
)

// HTTPLikeMessage HTTP消息
type HTTPLikeMessage struct {
	IsRequest bool
	Method    string
	URL       string
	Version   string
	Headers   map[string]string
	Body      []byte

	// 零拷贝字段
	RawHeaders []byte
	RawBody    []byte
}

// NewHTTPLikeParser 创建HTTP解析器
func NewHTTPLikeParser() *HTTPLikeParser {
	return &HTTPLikeParser{
		requestBuffer:  make([]byte, 0, 8192),
		responseBuffer: make([]byte, 0, 8192),
		MessageChan:    make(chan *HTTPLikeMessage, 100),
	}
}

// Parse 解析HTTP数据
func (hp *HTTPLikeParser) Parse(data []byte, isClient bool) {
	if isClient {
		hp.requestBuffer = append(hp.requestBuffer, data...)
		hp.parseRequest()
	} else {
		hp.responseBuffer = append(hp.responseBuffer, data...)
		hp.parseResponse()
	}
}

// parseRequest 解析HTTP请求
func (hp *HTTPLikeParser) parseRequest() {
	for {
		if len(hp.requestBuffer) == 0 {
			break
		}

		msg := hp.parseMessage(hp.requestBuffer, true)
		if msg == nil {
			break // 数据不完整
		}

		// 发送解析结果
		select {
		case hp.MessageChan <- msg:
		default:
			// 通道满，丢弃消息
		}

		// 移除已解析的数据
		consumed := len(msg.RawHeaders) + len(msg.RawBody)
		hp.requestBuffer = hp.requestBuffer[consumed:]
	}
}

// parseMessage 解析HTTP消息（零拷贝）
func (hp *HTTPLikeParser) parseMessage(buffer []byte, isRequest bool) *HTTPLikeMessage {
	// 查找HTTP头结束位置
	headerEnd := bytes.Index(buffer, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		return nil // 头部不完整
	}

	headerData := buffer[:headerEnd]
	bodyStart := headerEnd + 4

	// 解析头部
	msg := &HTTPLikeMessage{
		IsRequest:  isRequest,
		Headers:    make(map[string]string),
		RawHeaders: headerData, // 零拷贝引用
	}

	// 解析请求行/状态行
	lines := bytes.Split(headerData, []byte("\r\n"))
	if len(lines) == 0 {
		return nil
	}

	if isRequest {
		parts := bytes.Split(lines[0], []byte(" "))
		if len(parts) >= 3 {
			msg.Method = string(parts[0])
			msg.URL = string(parts[1])
			msg.Version = string(parts[2])
		}
	}

	// 解析头部字段
	var contentLength int
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if len(line) == 0 {
			continue
		}

		colonIndex := bytes.Index(line, []byte(":"))
		if colonIndex == -1 {
			continue
		}

		key := string(bytes.TrimSpace(line[:colonIndex]))
		value := string(bytes.TrimSpace(line[colonIndex+1:]))
		msg.Headers[strings.ToLower(key)] = value

		// 检查Content-Length
		if strings.ToLower(key) == "content-length" {
			contentLength = parseInt(value)
		}
	}

	// 处理消息体
	if contentLength > 0 {
		if len(buffer) < bodyStart+contentLength {
			return nil // 消息体不完整
		}
		msg.RawBody = buffer[bodyStart : bodyStart+contentLength]
		msg.Body = msg.RawBody // 零拷贝引用
	}

	return msg
}

// parseInt 快速整数解析
func parseInt(s string) int {
	num := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			num = num*10 + int(s[i]-'0')
		} else {
			break
		}
	}
	return num
}
