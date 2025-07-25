package capture

import (
	"fmt"
	"net"
	"time"

	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/utils"
	"github.com/go-playground/validator/v10"
	"github.com/google/gopacket/pcap"
	"golang.org/x/net/bpf"
)

// CaptureConfig 配置结构
type CaptureConfig struct {
	Interface    string                    `mapstructure:"interface" yaml:"interface" validate:"required,interface" json:"interface"`        // 网络接口名称
	SnapLen      int                       `mapstructure:"snap_len" yaml:"snap_len" validate:"min=64,max=65536" json:"snap_len"`             // 抓包长度
	TCPWorkers   int                       `mapstructure:"tcp_workers" yaml:"tcp_workers" validate:"min=1,max=100" json:"tcp_workers"`       // tcp工作协程数量
	UDPWorkers   int                       `mapstructure:"udp_workers" yaml:"udp_workers" validate:"min=1,max=100" json:"udp_workers"`       // udp工作协程数量
	BlockSize    int                       `mapstructure:"block_size" yaml:"block_size" validate:"min=4096,power_of_two" json:"block_size"`  // AF_PACKET块大小
	NumBlocks    int                       `mapstructure:"num_blocks" yaml:"num_blocks" validate:"min=1,max=1024" json:"num_blocks"`         // AF_PACKET块数量
	FlushTimeout time.Duration             `mapstructure:"flush_timeout" yaml:"flush_timeout" validate:"min=0,max=30s" json:"flush_timeout"` // 超时时间
	Filter       []bpf.RawInstruction      `mapstructure:"filter" yaml:"filter" validate:"max=100" json:"filter"`                            // 过滤规则
	handler      func(*types.RawFrameData) `validate:"-" json:"-"`                                                                           // 数据处理函数
}

// ValidationError 包含详细的验证错误信息
type ValidationError struct {
	Field   string      `json:"field"`
	Tag     string      `json:"tag"`
	Value   interface{} `json:"value"`
	Message string      `json:"message"`
}

func (v ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s (value: %v)", v.Field, v.Message, v.Value)
}

// ConfigValidationErrors 多个验证错误的集合
type ConfigValidationErrors []ValidationError

func (c ConfigValidationErrors) Error() string {
	if len(c) == 0 {
		return "no validation errors"
	}
	if len(c) == 1 {
		return c[0].Error()
	}
	return fmt.Sprintf("validation failed with %d errors: %s (and %d more)", len(c), c[0].Error(), len(c)-1)
}

// validator 全局验证器实例
var configValidator *validator.Validate

// 初始化验证器和自定义验证规则
func init() {
	configValidator = validator.New()

	// 注册自定义验证规则
	configValidator.RegisterValidation("interface", validateInterface)
	configValidator.RegisterValidation("power_of_two", validatePowerOfTwo)
}

// validateInterface 验证网络接口是否存在
func validateInterface(fl validator.FieldLevel) bool {
	interfaceName := fl.Field().String()

	// 特殊值检查
	if interfaceName == "any" || interfaceName == "lo" {
		return true
	}

	// 检查接口是否存在
	interfaces, err := net.Interfaces()
	if err != nil {
		return false
	}

	for _, iface := range interfaces {
		if iface.Name == interfaceName {
			return true
		}
	}

	return false
}

// validatePowerOfTwo 验证是否为2的幂
func validatePowerOfTwo(fl validator.FieldLevel) bool {
	value := fl.Field().Int()
	return value > 0 && (value&(value-1)) == 0
}

// Validate 验证配置
func (c *CaptureConfig) Validate() error {
	err := configValidator.Struct(c)
	if err == nil {
		return nil
	}

	var validationErrors ConfigValidationErrors

	if validatorErrors, ok := err.(validator.ValidationErrors); ok {
		for _, fieldError := range validatorErrors {
			validationErrors = append(validationErrors, ValidationError{
				Field:   fieldError.Field(),
				Tag:     fieldError.Tag(),
				Value:   fieldError.Value(),
				Message: getValidationMessage(fieldError),
			})
		}
	}

	return validationErrors
}

// getValidationMessage 根据验证标签返回友好的错误消息
func getValidationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "field is required"
	case "min":
		return fmt.Sprintf("value must be at least %s", fe.Param())
	case "max":
		return fmt.Sprintf("value must be at most %s", fe.Param())
	case "interface":
		return "network interface does not exist"
	case "power_of_two":
		return "value must be a power of 2"
	default:
		return fmt.Sprintf("validation failed with tag '%s'", fe.Tag())
	}
}

// ValidateAndApplyDefaults 验证配置并应用默认值
func (c *CaptureConfig) ValidateAndApplyDefaults() error {
	// 应用默认值
	c.applyDefaults()

	// 验证配置
	return c.Validate()
}

// applyDefaults 应用默认值
func (c *CaptureConfig) applyDefaults() {
	if c.Interface == "" {
		c.Interface = "any"
	}
	if c.SnapLen == 0 {
		c.SnapLen = 65536
	}
	if c.TCPWorkers == 0 {
		c.TCPWorkers = 4
	}
	if c.UDPWorkers == 0 {
		c.UDPWorkers = 4
	}
	if c.BlockSize == 0 {
		c.BlockSize = 1024 * 1024
	}
	if c.NumBlocks == 0 {
		c.NumBlocks = 128
	}
	if c.FlushTimeout == 0 {
		c.FlushTimeout = pcap.BlockForever
	}
	if len(c.Filter) == 0 {
		bpfFilter, _ := utils.CompileFilterWithDefaults("tcp or udp")
		c.Filter = bpfFilter
	}
}

// DefaultCaptureConfig 默认配置
func DefaultCaptureConfig() *CaptureConfig {
	config := &CaptureConfig{}
	config.applyDefaults()
	return config
}

// NewCaptureConfig 创建新的配置并验证
func NewCaptureConfig() (*CaptureConfig, error) {
	config := DefaultCaptureConfig()
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("default config validation failed: %w", err)
	}
	return config, nil
}

// Clone 克隆配置
func (c *CaptureConfig) Clone() *CaptureConfig {
	clone := *c
	// 深拷贝Filter切片
	if len(c.Filter) > 0 {
		clone.Filter = make([]bpf.RawInstruction, len(c.Filter))
		copy(clone.Filter, c.Filter)
	}
	return &clone
}

// String 返回配置的字符串表示
func (c *CaptureConfig) String() string {
	return fmt.Sprintf("CaptureConfig{Interface:%s, SnapLen:%d, TCPWorkers:%d, UDPWorkers:%d}",
		c.Interface, c.SnapLen, c.TCPWorkers, c.UDPWorkers)
}

// IsValid 快速检查配置是否有效
func (c *CaptureConfig) IsValid() bool {
	return c.Validate() == nil
}

// GetAvailableInterfaces 获取可用的网络接口列表
func GetAvailableInterfaces() ([]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var names []string
	names = append(names, "any", "lo") // 添加特殊接口

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp != 0 { // 只返回启用的接口
			names = append(names, iface.Name)
		}
	}

	return names, nil
}
