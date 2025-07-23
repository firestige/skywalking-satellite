package utils

import (
	"fmt"

	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"golang.org/x/net/bpf"
)

// BPFCompiler BPF编译器 - 专为以太网接口设计
type BPFCompiler struct {
	snaplen int
}

// NewBPFCompiler 创建BPF编译器 (固定使用以太网类型)
func NewBPFCompiler(snaplen int) *BPFCompiler {
	return &BPFCompiler{
		snaplen: snaplen,
	}
}

// CompileFilter 编译tcpdump格式的过滤器字符串为BPF指令
func (c *BPFCompiler) CompileFilter(filterStr string) ([]bpf.RawInstruction, error) {
	if filterStr == "" {
		return nil, nil
	}

	// 固定使用以太网链路类型
	bpfInstructions, err := pcap.CompileBPFFilter(layers.LinkTypeEthernet, c.snaplen, filterStr)
	if err != nil {
		return nil, fmt.Errorf("failed to compile BPF filter '%s': %v", filterStr, err)
	}

	// 转换为golang.org/x/net/bpf格式
	rawInstructions := make([]bpf.RawInstruction, len(bpfInstructions))
	for i, instr := range bpfInstructions {
		rawInstructions[i] = bpf.RawInstruction{
			Op: instr.Code,
			Jt: instr.Jt,
			Jf: instr.Jf,
			K:  instr.K,
		}
	}

	return rawInstructions, nil
}

// ValidateFilter 验证过滤器语法是否正确
func (c *BPFCompiler) ValidateFilter(filterStr string) error {
	_, err := c.CompileFilter(filterStr)
	return err
}

// CompileFilterWithDefaults 使用默认参数编译过滤器
func CompileFilterWithDefaults(filterStr string) ([]bpf.RawInstruction, error) {
	compiler := NewBPFCompiler(65536)
	return compiler.CompileFilter(filterStr)
}

// GetCommonFilters 获取常用的以太网过滤器模板
func GetCommonFilters() map[string]string {
	return map[string]string{
		"tcp_only":     "tcp",
		"udp_only":     "udp",
		"http_traffic": "tcp port 80 or tcp port 443",
		"dns_traffic":  "udp port 53 or tcp port 53",
		"ssh_traffic":  "tcp port 22",
		"web_traffic":  "tcp port 80 or tcp port 443 or tcp port 8080",
		"no_loopback":  "not host 127.0.0.1",
		"local_only":   "src net 192.168.0.0/16 or dst net 192.168.0.0/16",
		// 以太网特有的过滤器
		"broadcast":  "ether broadcast",
		"multicast":  "ether multicast",
		"to_gateway": "ether dst 00:00:5e:00:01:01", // 示例网关MAC
	}
}

// CombineFilters 组合多个过滤器
func CombineFilters(filters []string, operator string) string {
	if len(filters) == 0 {
		return ""
	}
	if len(filters) == 1 {
		return filters[0]
	}

	result := "(" + filters[0] + ")"
	for i := 1; i < len(filters); i++ {
		result += " " + operator + " (" + filters[i] + ")"
	}
	return result
}
