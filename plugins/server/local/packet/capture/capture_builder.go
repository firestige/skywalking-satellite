package capture

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
)

// NetworkCaptureBuilder 构建器
type NetworkCaptureBuilder struct {
	config    *CaptureConfig
	bpfFilter string
	ctx       context.Context
}

// NewNetworkCaptureBuilder 创建构建器
func NewNetworkCaptureBuilder(ctx context.Context, fn func(*types.RawFrameData)) *NetworkCaptureBuilder {
	defaultConfig := DefaultCaptureConfig()
	if fn != nil {
		defaultConfig.handler = fn
	}
	return &NetworkCaptureBuilder{
		config: defaultConfig,
		ctx:    ctx,
	}
}

// WithInterface 设置网络接口
func (b *NetworkCaptureBuilder) WithInterface(iface string) *NetworkCaptureBuilder {
	b.config.Interface = iface
	return b
}

// WithBPFFilter 设置BPF过滤器
func (b *NetworkCaptureBuilder) WithFilter(filter string) *NetworkCaptureBuilder {
	b.bpfFilter = filter
	return b
}

// WithWorkerCount 设置工作协程数量
func (b *NetworkCaptureBuilder) WithTCPWorker(count int) *NetworkCaptureBuilder {
	b.config.TCPWorkers = count
	return b
}

func (b *NetworkCaptureBuilder) WithUDPWorker(count int) *NetworkCaptureBuilder {
	b.config.UDPWorkers = count
	return b
}

// Build 构建DataSource
func (b *NetworkCaptureBuilder) Build() (types.DataSource, error) {
	// 验证配置
	if b.config.Interface == "" {
		return nil, fmt.Errorf("interface is required")
	}

	return newNetworkCapture(b.config, b.ctx), nil
}

// String 返回config的JSON字符串
func (b *NetworkCaptureBuilder) String() string {
	if b.config == nil {
		return "{}"
	}

	jsonBytes, err := json.Marshal(b.config)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to marshal config: %s"}`, err.Error())
	}

	return string(jsonBytes)
}
