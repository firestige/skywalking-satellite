package capture

import (
	"time"

	"github.com/apache/skywalking-satellite/plugins/server/local/packet/utils"
	"github.com/google/gopacket/pcap"
	"golang.org/x/net/bpf"
)

// CaptureConfig 配置结构
type CaptureConfig struct {
	Interface      string               `mapstructure:"interface" yaml:"interface"`             // 网络接口名称
	LocalAddresses []string             `mapstructure:"local_addresses" yaml:"local_addresses"` // 本机地址列表
	SnapLen        int                  `mapstructure:"snap_len" yaml:"snap_len"`               // 抓包长度
	RingSize       int                  `mapstructure:"ring_size" yaml:"ring_size"`             // 环形缓冲区大小
	WorkerCount    int                  `mapstructure:"worker_count" yaml:"worker_count"`       // 工作协程数量
	MTU            int                  `mapstructure:"mtu" yaml:"mtu"`                         // 最大传输单元
	BlockSize      int                  `mapstructure:"block_size" yaml:"block_size"`           // AF_PACKET块大小
	NumBlocks      int                  `mapstructure:"num_blocks" yaml:"num_blocks"`           // AF_PACKET块数量
	FlushTimeout   time.Duration        `mapstructure:"flush_timeout" yaml:"flush_timeout"`     // 超时时间
	Filter         []bpf.RawInstruction `mapstructure:"filter" yaml:"filter"`                   // 过滤规则
}

// DefaultCaptureConfig 默认配置
// todo builder写的有问题，配置没生效，生效的是这里的配置
func DefaultCaptureConfig() *CaptureConfig {
	bpfFilter, _ := utils.CompileFilterWithDefaults("tcp and udp")
	return &CaptureConfig{
		Interface:      "eth0",
		SnapLen:        65536,
		LocalAddresses: GetLocalAddresses(),
		RingSize:       1024,
		WorkerCount:    4,
		MTU:            1500,
		BlockSize:      1024 * 1024,
		NumBlocks:      128,
		FlushTimeout:   pcap.BlockForever,
		Filter:         bpfFilter,
	}
}
