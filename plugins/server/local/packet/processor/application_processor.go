package processor

import (
	"context"
	"time"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
)

// ApplicationProcessor 应用层处理器
type ApplicationProcessor struct {
	httpReassembler *TCPStreamReassembler
	grpcReassembler *TCPStreamReassembler

	// 输出通道
	outputChan chan *CompleteMessage

	ctx context.Context
}

// NewApplicationProcessor 创建应用层处理器
func NewApplicationProcessor(ctx context.Context) *ApplicationProcessor {
	processor := &ApplicationProcessor{
		httpReassembler: NewTCPStreamReassembler(NewHTTPBoundaryDetector()),
		grpcReassembler: NewTCPStreamReassembler(NewgRPCBoundaryDetector()),
		outputChan:      make(chan *CompleteMessage, 2000),
		ctx:             ctx,
	}

	// 启动消息转发协程
	go processor.startMessageForwarding()

	// 启动清理协程
	go processor.startCleanupRoutine()

	return processor
}

// ProcessFrameFromCapture 处理来自capture层的帧数据
func (p *ApplicationProcessor) ProcessFrameFromCapture(frame *types.RawFrameData) {
	if frame.Connection.Protocol != "TCP" {
		return // 只处理TCP流
	}

	// 根据端口判断协议类型
	protocol := p.detectProtocol(frame)

	switch protocol {
	case "HTTP":
		p.httpReassembler.ProcessFrame(frame)
	case "gRPC":
		p.grpcReassembler.ProcessFrame(frame)
	default:
		// 未知协议，可以使用通用处理器或忽略
		log.Logger.Debugf("Unknown protocol for port %d", frame.Connection.DstPort)
	}
}

// detectProtocol 根据端口和内容检测协议
func (p *ApplicationProcessor) detectProtocol(frame *types.RawFrameData) string {
	// 简单的协议检测逻辑
	port := frame.Connection.DstPort

	// 基于端口的初步判断
	switch port {
	case 80, 8080, 8000, 3000:
		return "HTTP"
	case 443, 8443:
		// HTTPS，需要更复杂的检测
		return "HTTP"
	case 9090, 50051:
		return "gRPC"
	default:
		// 基于内容的检测
		return p.detectProtocolByContent(frame.Data)
	}
}

// detectProtocolByContent 基于内容检测协议
func (p *ApplicationProcessor) detectProtocolByContent(data []byte) string {
	if len(data) == 0 {
		return "Unknown"
	}

	// HTTP特征检测
	httpMethods := [][]byte{
		[]byte("GET "), []byte("POST "), []byte("PUT "),
		[]byte("DELETE "), []byte("HEAD "), []byte("OPTIONS "),
	}

	for _, method := range httpMethods {
		if len(data) >= len(method) &&
			string(data[:len(method)]) == string(method) {
			return "HTTP"
		}
	}

	// HTTP响应特征
	if len(data) >= 4 && string(data[:4]) == "HTTP" {
		return "HTTP"
	}

	// gRPC特征检测（简化版）
	if len(data) >= 5 {
		// gRPC帧以压缩标志(1字节)和长度(4字节)开始
		// 这里做简单的启发式检测
		compressed := data[0]
		if compressed <= 1 { // 压缩标志应该是0或1
			length := int(data[1])<<24 | int(data[2])<<16 | int(data[3])<<8 | int(data[4])
			if length > 0 && length < 1024*1024 { // 合理的消息长度
				return "gRPC"
			}
		}
	}

	return "Unknown"
}

// startMessageForwarding 启动消息转发
func (p *ApplicationProcessor) startMessageForwarding() {
	httpChan := p.httpReassembler.GetMessageChannel()
	grpcChan := p.grpcReassembler.GetMessageChannel()

	for {
		select {
		case <-p.ctx.Done():
			return
		case msg := <-httpChan:
			p.forwardMessage(msg)
		case msg := <-grpcChan:
			p.forwardMessage(msg)
		}
	}
}

// forwardMessage 转发完整消息
func (p *ApplicationProcessor) forwardMessage(msg *CompleteMessage) {
	select {
	case p.outputChan <- msg:
		log.Logger.Debugf("Forwarded %s message: %d bytes", msg.Protocol, len(msg.Data))
	default:
		log.Logger.Warn("Output channel full, dropping message")
	}
}

// startCleanupRoutine 启动清理例程
func (p *ApplicationProcessor) startCleanupRoutine() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.httpReassembler.CleanupTimeoutStreams()
			p.grpcReassembler.CleanupTimeoutStreams()
		}
	}
}

// GetOutputChannel 获取输出通道
func (p *ApplicationProcessor) GetOutputChannel() <-chan *CompleteMessage {
	return p.outputChan
}

// Close 关闭处理器
func (p *ApplicationProcessor) Close() {
	close(p.outputChan)
}
