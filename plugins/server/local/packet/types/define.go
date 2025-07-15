package types

import (
	"context"
	"sync"
)

type Lifecycle interface {
	Prepare() error
	Start(ctx context.Context, wg *sync.WaitGroup) error
	Close() error
}

type DataSource interface {
	Lifecycle
	Fetch(ctx context.Context) ([]byte, error)
}

type RawFrameData struct {
	Data       []byte
	Meta       map[string]string
	Connection Connection
	Timestamp  int64
	Direction  string // "inbound" or "outbound"
}

type Connection struct {
	SrcHost  string
	SrcPort  int
	DestHost string
	DstPort  int
	Protocol string
}

// StreamProcessor 流处理器接口
type StreamProcessor interface {
	Lifecycle
	// 添加数据到流中
	Feed(ctx context.Context, frame RawFrameData) error

	// 设置过滤器
	WithFilter(filter FilterFunc) StreamProcessor

	// 设置映射器
	WithMapper(mapper MapFunc) StreamProcessor

	// 设置聚合器（用于TCP流重组）
	WithAggregator(aggregator AggregatorFunc) StreamProcessor

	// 设置输出处理器
	WithSink(sink SinkFunc) StreamProcessor

	// 停止流处理
	Stop() error
}

// 函数类型定义
type FilterFunc func(frame RawFrameData) bool
type MapFunc func(frame RawFrameData) RawFrameData
type SinkFunc func(frame RawFrameData) error

// AggregatorFunc 聚合器函数，用于TCP流重组
type AggregatorFunc func(frames []RawFrameData) ([]RawFrameData, error)

type Dispatcher interface {
	SinkFunc
	Dispatch(ctx context.Context, frame RawFrameData) error
}

type FrameHandler interface {
	Handle(ctx context.Context, frame RawFrameData) error
	Name() string
	IsSupported(frame RawFrameData) bool
}

type HandlerManager interface {
	Lifecycle
	RegisterHandler(handler FrameHandler) error
	GetHandler(name string) (FrameHandler, bool)
	UnregisterHandler(name string) error
	GetAllHandlers() []FrameHandler
}
