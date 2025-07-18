package sip

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type SipParser struct{}

func (p *SipParser) Parse(data []byte) (SipMessage, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	reader := bufio.NewReader(bytes.NewReader(data))

	// 读取第一行判断是请求还是响应
	firstLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read first line: %w", err)
	}

	firstLine = strings.TrimSpace(firstLine)

	// 解析头部
	headers := make(map[string]string)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break // 空行表示头部结束
		}

		// 解析头部字段
		if colonIndex := strings.Index(line, ":"); colonIndex != -1 {
			key := strings.TrimSpace(line[:colonIndex])
			value := strings.TrimSpace(line[colonIndex+1:])
			headers[key] = value
		}
	}

	// 读取消息体
	var bodyBuffer bytes.Buffer
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		bodyBuffer.WriteString(line)
	}
	body := strings.TrimSpace(bodyBuffer.String())

	// 提取通用字段
	callId := headers["Call-ID"]
	cSeq := headers["CSeq"]

	// 判断是请求还是响应
	if strings.HasPrefix(firstLine, "SIP/2.0") {
		// 这是一个响应
		return p.parseResponse(firstLine, headers, body, callId, cSeq)
	} else {
		// 这是一个请求
		return p.parseRequest(firstLine, headers, body, callId, cSeq)
	}
}

func (p *SipParser) parseRequest(requestLine string, headers map[string]string, body, callId, cSeq string) (SipMessage, error) {
	// 解析请求行: METHOD sip:uri SIP/2.0
	parts := strings.Fields(requestLine)
	if len(parts) < 3 {
		return nil, errors.New("invalid request line format")
	}

	method := parts[0]

	sipMsg := &sipMessage{
		callId:  callId,
		cSeq:    cSeq,
		headers: headers,
		body:    body,
		request: true,
	}

	return &sipRequest{
		sipMessage:  *sipMsg,
		method:      method,
		requestLine: requestLine,
	}, nil
}

func (p *SipParser) parseResponse(statusLine string, headers map[string]string, body, callId, cSeq string) (SipMessage, error) {
	// 解析状态行: SIP/2.0 status_code reason_phrase
	parts := strings.Fields(statusLine)
	if len(parts) < 3 {
		return nil, errors.New("invalid status line format")
	}

	statusCode, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid status code: %w", err)
	}

	sipMsg := &sipMessage{
		callId:  callId,
		cSeq:    cSeq,
		headers: headers,
		body:    body,
		request: false,
	}

	return &sipResponse{
		sipMessage: *sipMsg,
		status:     statusCode,
		statusLine: statusLine,
	}, nil
}

// NewSipParser 创建新的SIP解析器实例
func NewSipParser() *SipParser {
	return &SipParser{}
}

// ParseMultiple 解析多个SIP消息（当数据包含多个消息时）
func (p *SipParser) ParseMultiple(data []byte) ([]SipMessage, error) {
	var messages []SipMessage

	// 使用双换行符分割多个SIP消息
	parts := bytes.Split(data, []byte("\r\n\r\n"))

	for _, part := range parts {
		if len(bytes.TrimSpace(part)) == 0 {
			continue
		}

		// 重新添加结束标记
		fullMessage := append(part, []byte("\r\n\r\n")...)

		msg, err := p.Parse(fullMessage)
		if err != nil {
			continue // 跳过无法解析的消息
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

// IsValidSipMessage 检查数据是否为有效的SIP消息
func (p *SipParser) IsValidSipMessage(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	dataStr := string(data)

	// 检查是否以SIP请求方法开头或SIP/2.0响应开头
	sipMethods := []string{"INVITE", "ACK", "BYE", "CANCEL", "REGISTER", "OPTIONS", "PRACK", "SUBSCRIBE", "NOTIFY", "PUBLISH", "INFO", "REFER", "MESSAGE", "UPDATE"}

	for _, method := range sipMethods {
		if strings.HasPrefix(dataStr, method+" ") {
			return true
		}
	}

	return strings.HasPrefix(dataStr, "SIP/2.0")
}
