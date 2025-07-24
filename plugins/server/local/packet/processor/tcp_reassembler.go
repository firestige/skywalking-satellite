package processor

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
)

// MessageBoundary 消息边界检测接口
type MessageBoundary interface {
	// DetectBoundary 检测消息边界，返回完整消息的长度，-1表示不完整
	DetectBoundary(data []byte) int
	// GetProtocolName 获取协议名称
	GetProtocolName() string
}

// TCPStreamReassembler TCP流重组器（应用层实现）
type TCPStreamReassembler struct {
	streams  map[string]*tcpStream
	mutex    sync.RWMutex
	boundary MessageBoundary

	// 输出通道
	messageChan chan *CompleteMessage

	// 配置
	maxStreamSize int
	streamTimeout time.Duration
}

// tcpStream TCP流状态
type tcpStream struct {
	connKey    string
	buffer     *bytes.Buffer
	lastUpdate time.Time
	direction  string
	connection types.Connection // 使用兼容的连接类型
}

// CompleteMessage 完整消息结构
type CompleteMessage struct {
	Data       []byte
	Connection types.Connection
	Protocol   string
	Timestamp  int64
	Direction  string
	Meta       map[string]string
}

// NewTCPStreamReassembler 创建TCP流重组器
func NewTCPStreamReassembler(boundary MessageBoundary) *TCPStreamReassembler {
	return &TCPStreamReassembler{
		streams:       make(map[string]*tcpStream),
		boundary:      boundary,
		messageChan:   make(chan *CompleteMessage, 1000),
		maxStreamSize: 1024 * 1024, // 1MB
		streamTimeout: 5 * time.Minute,
	}
}

// ProcessFrame 处理来自capture层的帧数据
func (r *TCPStreamReassembler) ProcessFrame(frame *types.RawFrameData) {
	if frame.Connection.Protocol != "TCP" {
		return // 只处理TCP
	}

	connKey := r.getConnectionKey(frame.Connection)

	r.mutex.Lock()
	stream := r.getOrCreateStream(connKey, frame)
	r.mutex.Unlock()

	// 将数据添加到流缓冲区
	stream.buffer.Write(frame.Data)
	stream.lastUpdate = time.Now()

	// 尝试提取完整消息
	r.extractMessages(stream)
}

// extractMessages 从流中提取完整消息
func (r *TCPStreamReassembler) extractMessages(stream *tcpStream) {
	for {
		data := stream.buffer.Bytes()
		if len(data) == 0 {
			break
		}

		// 检测消息边界
		messageLen := r.boundary.DetectBoundary(data)
		if messageLen <= 0 {
			break // 消息不完整
		}

		// 提取完整消息
		messageData := make([]byte, messageLen)
		stream.buffer.Read(messageData)

		// 创建完整消息
		message := &CompleteMessage{
			Data:       messageData,
			Connection: stream.connection,
			Protocol:   r.boundary.GetProtocolName(),
			Timestamp:  time.Now().UnixNano() / 1e6,
			Direction:  stream.direction,
			Meta: map[string]string{
				"stream_key":     stream.connKey,
				"message_length": fmt.Sprintf("%d", messageLen),
				"reassembled":    "true",
			},
		}

		// 发送到输出通道
		select {
		case r.messageChan <- message:
		default:
			// 通道满，丢弃消息
		}
	}
}

// getConnectionKey 生成连接键
func (r *TCPStreamReassembler) getConnectionKey(conn types.Connection) string {
	return fmt.Sprintf("%s:%d->%s:%d",
		conn.SrcHost, conn.SrcPort,
		conn.DestHost, conn.DstPort)
}

// getOrCreateStream 获取或创建流
func (r *TCPStreamReassembler) getOrCreateStream(connKey string, frame *types.RawFrameData) *tcpStream {
	stream, exists := r.streams[connKey]
	if !exists {
		stream = &tcpStream{
			connKey:    connKey,
			buffer:     bytes.NewBuffer(nil),
			lastUpdate: time.Now(),
			direction:  frame.Direction,
			connection: frame.Connection,
		}
		r.streams[connKey] = stream
	}
	return stream
}

// GetMessageChannel 获取消息输出通道
func (r *TCPStreamReassembler) GetMessageChannel() <-chan *CompleteMessage {
	return r.messageChan
}

// CleanupTimeoutStreams 清理超时流
func (r *TCPStreamReassembler) CleanupTimeoutStreams() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now()
	for key, stream := range r.streams {
		if now.Sub(stream.lastUpdate) > r.streamTimeout {
			delete(r.streams, key)
		}
	}
}
