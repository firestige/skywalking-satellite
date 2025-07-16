package packet

import (
	"bytes"
	"time"

	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/utils"
)

// LengthFieldBaseDecoder 基于长度字段的解码器
type LengthFieldBaseDecoder struct {
	// 使用 RingBuffer 替代普通缓冲区
	ringBuffer *utils.RingBuffer

	// 当前处理的数据缓冲区
	currentBuffer []byte

	// 解析状态
	state ParseState

	// 当前帧的长度信息
	headerEndPos  int
	contentLength int

	// 连接信息（用于填充 RawFrameData）
	connection types.Connection

	// 结果通道
	FrameChan chan *types.RawFrameData
}

type ParseState int

const (
	StateParsingHeaders ParseState = iota
	StateParsingBody
	StateFrameComplete
)

// NewLengthFieldBaseDecoder 创建基于长度字段的解码器
func NewLengthFieldBaseDecoder(connection types.Connection) *LengthFieldBaseDecoder {
	return &LengthFieldBaseDecoder{
		ringBuffer:    utils.NewRingBuffer(1024), // 1024个槽位
		currentBuffer: make([]byte, 0, 8192),
		state:         StateParsingHeaders,
		connection:    connection,
		FrameChan:     make(chan *types.RawFrameData, 100),
	}
}

// Feed 向解码器输入数据
func (d *LengthFieldBaseDecoder) Feed(data []byte, direction string) {
	// 使用 RingBuffer 的零拷贝写入
	dataPtr := d.ringBuffer.ZeroCopyWrite(data)
	if dataPtr != nil {
		// 将数据追加到当前缓冲区
		d.currentBuffer = append(d.currentBuffer, *dataPtr...)
		d.processBuffer(direction)
	}
}

// processBuffer 处理缓冲区中的数据
func (d *LengthFieldBaseDecoder) processBuffer(direction string) {
	for {
		switch d.state {
		case StateParsingHeaders:
			if !d.parseHeaders() {
				return // 头部数据不完整
			}
			d.state = StateParsingBody

		case StateParsingBody:
			if !d.parseBody() {
				return // 消息体数据不完整
			}
			d.state = StateFrameComplete

		case StateFrameComplete:
			d.extractFrame(direction)
			d.resetState()
			d.state = StateParsingHeaders

			// 如果缓冲区还有数据，继续处理
			if len(d.currentBuffer) == 0 {
				return
			}
		}
	}
}

// parseHeaders 解析头部，寻找长度字段
func (d *LengthFieldBaseDecoder) parseHeaders() bool {
	// 查找头部结束标记
	headerEnd := bytes.Index(d.currentBuffer, []byte("\r\n\r\n"))
	if headerEnd == -1 {
		return false // 头部不完整
	}

	d.headerEndPos = headerEnd + 4
	d.contentLength = d.extractContentLength(d.currentBuffer[:headerEnd])

	return true
}

// parseBody 解析消息体
func (d *LengthFieldBaseDecoder) parseBody() bool {
	totalFrameLength := d.headerEndPos + d.contentLength

	// 检查是否有足够的数据
	if len(d.currentBuffer) < totalFrameLength {
		return false // 消息体不完整
	}

	return true
}

// extractFrame 提取完整的帧
func (d *LengthFieldBaseDecoder) extractFrame(direction string) {
	totalFrameLength := d.headerEndPos + d.contentLength

	// 创建 RawFrameData，复制数据以避免引用问题
	frameData := make([]byte, totalFrameLength)
	copy(frameData, d.currentBuffer[:totalFrameLength])

	// 创建元数据
	meta := make(map[string]string)
	meta["frame_length"] = string(rune(totalFrameLength))
	meta["header_length"] = string(rune(d.headerEndPos))
	meta["content_length"] = string(rune(d.contentLength))

	// 使用 types.RawFrameData 结构
	frame := &types.RawFrameData{
		Data:       frameData,
		Meta:       meta,
		Connection: d.connection,
		Timestamp:  time.Now().UnixNano(),
		Direction:  direction,
	}

	// 发送到通道
	select {
	case d.FrameChan <- frame:
	default:
		// 通道满，丢弃帧
	}

	// 移除已处理的数据
	d.currentBuffer = d.currentBuffer[totalFrameLength:]
}

// resetState 重置解析状态
func (d *LengthFieldBaseDecoder) resetState() {
	d.headerEndPos = 0
	d.contentLength = 0
}

// extractContentLength 从头部提取内容长度
func (d *LengthFieldBaseDecoder) extractContentLength(headerData []byte) int {
	lines := bytes.Split(headerData, []byte("\r\n"))

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		colonIndex := bytes.Index(line, []byte(":"))
		if colonIndex == -1 {
			continue
		}

		key := bytes.TrimSpace(line[:colonIndex])
		value := bytes.TrimSpace(line[colonIndex+1:])

		// 检查各种长度字段（不区分大小写）
		if bytes.EqualFold(key, []byte("content-length")) {
			return parseInt(string(value))
		}
	}

	return 0 // 没有找到长度字段，假设没有消息体
}

// parseInt 快速整数解析
func parseInt(s string) int {
	if len(s) == 0 {
		return 0
	}

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

// Close 关闭解码器
func (d *LengthFieldBaseDecoder) Close() {
	close(d.FrameChan)
}

// GetStats 获取统计信息
func (d *LengthFieldBaseDecoder) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"buffer_size":    len(d.currentBuffer),
		"state":          d.state,
		"header_end_pos": d.headerEndPos,
		"content_length": d.contentLength,
		"connection":     d.connection,
	}
}

// UpdateConnection 更新连接信息
func (d *LengthFieldBaseDecoder) UpdateConnection(connection types.Connection) {
	d.connection = connection
}
