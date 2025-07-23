package capture

import (
	"time"

	"github.com/google/gopacket/pcap"
	"golang.org/x/net/bpf"
)

// CaptureConfig 配置结构
type CaptureConfig struct {
	Interface      string               // 网络接口名称
	BPFFilter      []bpf.RawInstruction // BPF过滤规则
	SnapLen        int                  // 抓包长度
	RingSize       int                  // 环形缓冲区大小
	WorkerCount    int                  // 工作协程数量
	MTU            int                  // 最大传输单元
	BlockSize      int                  // AF_PACKET块大小
	NumBlocks      int                  // AF_PACKET块数量
	FlushTimeout   time.Duration        // 超时时间
	LocalAddresses []string             // 本机地址列表
}

type FilterRule struct {
	TCP []string `mapstructure:"tcp"` // TCP端口过滤规则
	UDP []string `mapstructure:"udp"` // UDP端口过滤规则
}

type ParseConfig struct {
	RingSize     int           `mapstructure:"ring_size"`     // 环形缓冲区大小
	WorkerCount  int           `mapstructure:"worker_count"`  // 工作协程数量
	MTU          int           `mapstructure:"mtu"`           // 最大传输单元
	BlockSize    int           `mapstructure:"block_size"`    // AF_PACKET块大小
	NumBlocks    int           `mapstructure:"num_blocks"`    // AF_PACKET块数量
	FlushTimeout time.Duration `mapstructure:"flush_timeout"` // 超时时间
}

type PacketConfig struct {
	Interface      string   `mapstructure:"interface"`       // 网络接口名称
	LocalAddresses []string `mapstructure:"local_addresses"` // 本机地址列表
	SnapLen        int      `mapstructure:"snap_len"`        // 抓包长度
}

// DefaultCaptureConfig 默认配置
// todo builder写的有问题，配置没生效，生效的是这里的配置
func DefaultCaptureConfig() *CaptureConfig {
	return &CaptureConfig{
		Interface:      "eth0",
		SnapLen:        65536,
		RingSize:       1024,
		WorkerCount:    4,
		MTU:            1500,
		BlockSize:      1024 * 1024,
		NumBlocks:      128,
		FlushTimeout:   pcap.BlockForever,
		LocalAddresses: GetLocalAddresses(),
	}
}
