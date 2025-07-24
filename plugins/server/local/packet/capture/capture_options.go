package capture

import (
	"fmt"
	"net"

	"github.com/apache/skywalking-satellite/plugins/server/local/packet/utils"
	"github.com/go-playground/validator/v10"
)

type Options struct {
	Interface      string   `validate:"required" json:"interface"`
	WorkerCount    int      `validate:"min=1,max=100" json:"worker_count"`
	RingSize       int      `validate:"min=1" json:"ring_size"`
	SnapLen        int      `validate:"min=1" json:"snap_len"`
	MTU            int      `validate:"min=1,max=9000" json:"mtu"`
	NumBlocks      int      `validate:"min=1" json:"num_blocks"`
	BlockSize      int      `validate:"min=1" json:"block_size"`
	FlushTimeout   int      `validate:"min=0" json:"flush_timeout"`
	Filter         string   `validate:"bpf_filter" json:"filter"`
	LocalAddresses []string `validate:"dive,local_address" json:"local_addresses"`
}

func DefaultOptions() *Options {
	return &Options{
		Interface:      "any",
		WorkerCount:    4,
		RingSize:       1024,
		SnapLen:        65536,
		MTU:            1500,
		NumBlocks:      128,
		BlockSize:      4096,
		FlushTimeout:   100,
		Filter:         "tcp or udp",
		LocalAddresses: utils.GetLocalAddresses(),
	}
}

func (o *Options) Validate() error {
	validate := validator.New()

	// 注册自定义验证器
	if err := validate.RegisterValidation("bpf_filter", validateBPFFilter); err != nil {
		return fmt.Errorf("failed to register bpf_filter validator: %w", err)
	}

	if err := validate.RegisterValidation("local_address", validateLocalAddress); err != nil {
		return fmt.Errorf("failed to register local_address validator: %w", err)
	}

	return validate.Struct(o)
}

// validateBPFFilter 使用现有的BPF基础设施验证过滤器
func validateBPFFilter(fl validator.FieldLevel) bool {
	filter := fl.Field().String()

	// 空过滤器是允许的
	if filter == "" {
		return true
	}

	// 使用现有的BPF编译器验证过滤器
	compiler := utils.NewBPFCompiler(65536)
	err := compiler.ValidateFilter(filter)
	return err == nil
}

// validateLocalAddress 验证本机地址
func validateLocalAddress(fl validator.FieldLevel) bool {
	address := fl.Field().String()

	// 解析IP地址
	ip := net.ParseIP(address)
	if ip == nil {
		return false
	}

	// 获取所有本机地址
	localAddresses := utils.GetLocalAddresses()

	// 检查是否为本机地址
	for _, localAddr := range localAddresses {
		if localAddr == address {
			return true
		}
	}

	// 检查是否为回环地址
	if ip.IsLoopback() {
		return true
	}

	// 检查是否为通配符地址（0.0.0.0 或 ::）
	if ip.IsUnspecified() {
		return true
	}

	return false
}
