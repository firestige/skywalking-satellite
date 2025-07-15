package esl

import (
	"bufio"
	"bytes"
	"fmt"
	"net/textproto"
	"strconv"
	"strings"
)

// ESL Event represents a FreeSWITCH event
type ESLEvent struct {
	Headers map[string]string
	Body    string
}

// ESLParser parses ESL protocol messages
type ESLParser struct{}

func (p *ESLParser) ParseMessage(data []byte) (*ESLEvent, error) {
	reader := textproto.NewReader(bufio.NewReader(bytes.NewReader(data)))

	// Read headers
	headers := make(map[string]string)
	for {
		line, err := reader.ReadLine()
		if err != nil {
			return nil, fmt.Errorf("failed to read line: %v", err)
		}

		if line == "" {
			break // End of headers
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			headers[key] = value
		}
	}

	// Read body if Content-Length is specified
	var body string
	if contentLengthStr, ok := headers["Content-Length"]; ok {
		contentLength, err := strconv.Atoi(contentLengthStr)
		if err != nil {
			return nil, fmt.Errorf("invalid Content-Length: %v", err)
		}

		if contentLength > 0 {
			bodyBytes := make([]byte, contentLength)
			_, err := reader.R.Read(bodyBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to read body: %v", err)
			}
			body = string(bodyBytes)
		}
	}

	return &ESLEvent{
		Headers: headers,
		Body:    body,
	}, nil
}
