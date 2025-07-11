package afpacket

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
)

// HTTPDecoder 用于在环形缓冲区中拼接和解析完整的 HTTP 报文
// 支持 Content-Length 的 HTTP 报文（不支持 chunked）
type HTTPDecoder struct {
	buf      []byte
	size     int
	start    int // 环形缓冲区起点
	end      int // 环形缓冲区终点
	occupied int // 已用字节数
}

// NewHTTPDecoder 创建一个指定大小的环形缓冲区
func NewHTTPDecoder(size int) *HTTPDecoder {
	return &HTTPDecoder{
		buf:  make([]byte, size),
		size: size,
	}
}

// Push 将数据片段写入环形缓冲区
func (d *HTTPDecoder) Push(data []byte) error {
	if len(data) > d.size-d.occupied {
		return errors.New("buffer overflow")
	}
	for _, b := range data {
		d.buf[d.end] = b
		d.end = (d.end + 1) % d.size
		d.occupied++
	}
	return nil
}

// NextPacket 尝试解析下一个完整 HTTP 报文，返回报文内容和是否成功
func (d *HTTPDecoder) NextPacket() ([]byte, bool) {
	if d.occupied == 0 {
		return nil, false
	}
	// 1. 找到 HTTP 首行（GET/POST/PUT/DELETE/HEAD/OPTIONS/HTTP/1.1等开头）
	headersEnd := -1
	var headerBuf bytes.Buffer
	var i, pos int
	for i = 0; i < d.occupied; i++ {
		pos = (d.start + i) % d.size
		headerBuf.WriteByte(d.buf[pos])
		if headerBuf.Len() >= 4 && bytes.HasSuffix(headerBuf.Bytes(), []byte("\r\n\r\n")) {
			headersEnd = i + 1
			break
		}
	}
	if headersEnd == -1 {
		return nil, false // 头未收全
	}
	headers := headerBuf.Bytes()
	// 2. 解析 Content-Length
	headersStr := string(headers)
	lines := strings.Split(headersStr, "\r\n")
	contentLen := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			cl := strings.TrimSpace(line[len("content-length:"):])
			if n, err := strconv.Atoi(cl); err == nil {
				contentLen = n
			}
		}
	}
	// 3. 计算总报文长度
	totalLen := headersEnd + contentLen
	if d.occupied < totalLen {
		return nil, false // 数据未收全
	}
	// 4. 拷贝完整报文
	packet := make([]byte, totalLen)
	for i = 0; i < totalLen; i++ {
		pos = (d.start + i) % d.size
		packet[i] = d.buf[pos]
	}
	// 5. 移动 start 指针，释放空间
	d.start = (d.start + totalLen) % d.size
	d.occupied -= totalLen
	return packet, true
}
