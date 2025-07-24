package processor

import (
	"bytes"
	"strconv"
	"strings"
)

// HTTPBoundaryDetector HTTP消息边界检测器
type HTTPBoundaryDetector struct{}

func NewHTTPBoundaryDetector() *HTTPBoundaryDetector {
	return &HTTPBoundaryDetector{}
}

func (h *HTTPBoundaryDetector) GetProtocolName() string {
	return "HTTP"
}

func (h *HTTPBoundaryDetector) DetectBoundary(data []byte) int {
	// 查找HTTP头部结束标记
	headerEnd := bytes.Index(data, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		return -1 // 头部不完整
	}

	headerEndPos := headerEnd + 4
	headers := string(data[:headerEnd])

	// 解析Content-Length
	contentLength := h.extractContentLength(headers)
	totalLength := headerEndPos + contentLength

	if len(data) < totalLength {
		return -1 // 消息体不完整
	}

	return totalLength
}

func (h *HTTPBoundaryDetector) extractContentLength(headers string) int {
	lines := strings.Split(headers, "\r\n")

	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				if length, err := strconv.Atoi(value); err == nil {
					return length
				}
			}
		}
	}

	return 0 // 没有Content-Length，假设没有消息体
}

// gRPCBoundaryDetector gRPC消息边界检测器
type gRPCBoundaryDetector struct{}

func NewgRPCBoundaryDetector() *gRPCBoundaryDetector {
	return &gRPCBoundaryDetector{}
}

func (g *gRPCBoundaryDetector) GetProtocolName() string {
	return "gRPC"
}

func (g *gRPCBoundaryDetector) DetectBoundary(data []byte) int {
	if len(data) < 5 {
		return -1 // gRPC帧头不完整
	}

	// gRPC帧格式: [1字节压缩标志][4字节长度][消息内容]
	// compressed := data[0]
	length := int(data[1])<<24 | int(data[2])<<16 | int(data[3])<<8 | int(data[4])

	totalLength := 5 + length
	if len(data) < totalLength {
		return -1 // 消息不完整
	}

	return totalLength
}

// FixedLengthBoundaryDetector 固定长度消息边界检测器
type FixedLengthBoundaryDetector struct {
	messageLength int
}

func NewFixedLengthBoundaryDetector(length int) *FixedLengthBoundaryDetector {
	return &FixedLengthBoundaryDetector{
		messageLength: length,
	}
}

func (f *FixedLengthBoundaryDetector) GetProtocolName() string {
	return "FixedLength"
}

func (f *FixedLengthBoundaryDetector) DetectBoundary(data []byte) int {
	if len(data) >= f.messageLength {
		return f.messageLength
	}
	return -1
}

// DelimiterBoundaryDetector 分隔符边界检测器
type DelimiterBoundaryDetector struct {
	delimiter []byte
	protocol  string
}

func NewDelimiterBoundaryDetector(delimiter []byte, protocol string) *DelimiterBoundaryDetector {
	return &DelimiterBoundaryDetector{
		delimiter: delimiter,
		protocol:  protocol,
	}
}

func (d *DelimiterBoundaryDetector) GetProtocolName() string {
	return d.protocol
}

func (d *DelimiterBoundaryDetector) DetectBoundary(data []byte) int {
	index := bytes.Index(data, d.delimiter)
	if index != -1 {
		return index + len(d.delimiter)
	}
	return -1
}
