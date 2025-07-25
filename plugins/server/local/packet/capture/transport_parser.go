package capture

import (
	"fmt"
	"strings"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func parseTransportLayers(packet gopacket.Packet, frame *types.RawFrameData) {
	ethLayer := packet.Layer(layers.LayerTypeEthernet)
	ipLayer := packet.Layer(layers.LayerTypeIPv4)

	if ethLayer == nil || ipLayer == nil {
		log.Logger.Debugf("Packet does not contain Ethernet or IPv4 layer: %s", packet)
		// todo add drop packet statistics
		return
	}

	ip4 := ipLayer.(*layers.IPv4)

	frame.Data = packet.Data()
	frame.Timestamp = packet.Metadata().Timestamp.UnixNano()
	frame.Meta = make(map[string]string)
	frame.Connection = types.Connection{
		SrcHost:  ip4.SrcIP.String(),
		DestHost: ip4.DstIP.String(),
		Protocol: ip4.Protocol,
	}

	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	udpLayer := packet.Layer(layers.LayerTypeUDP)

	if tcpLayer != nil {
		tcp := tcpLayer.(*layers.TCP)
		frame.Meta["protocol"] = "TCP"
		frame.Meta["flags"] = buildTCPFlagsString(tcp) // 使用自定义函数构建flags字符串
		frame.Connection.SrcPort = int(tcp.SrcPort)
		frame.Connection.DstPort = int(tcp.DstPort)
		frame.Direction = types.Unknown // Direction needs to be determined based on context
	} else if udpLayer != nil {
		udp := udpLayer.(*layers.UDP)
		frame.Meta["protocol"] = "UDP"
		frame.Meta["length"] = fmt.Sprintf("%d", udp.Length) // 修正length转换
		frame.Connection.SrcPort = int(udp.SrcPort)
		frame.Connection.DstPort = int(udp.DstPort)
		frame.Direction = types.Unknown // Direction needs to be determined based on context
	}
}

// buildTCPFlagsString 根据TCP的bool标志位构建flags字符串
func buildTCPFlagsString(tcp *layers.TCP) string {
	var flags []string

	if tcp.FIN {
		flags = append(flags, "FIN")
	}
	if tcp.SYN {
		flags = append(flags, "SYN")
	}
	if tcp.RST {
		flags = append(flags, "RST")
	}
	if tcp.PSH {
		flags = append(flags, "PSH")
	}
	if tcp.ACK {
		flags = append(flags, "ACK")
	}
	if tcp.URG {
		flags = append(flags, "URG")
	}
	if tcp.ECE {
		flags = append(flags, "ECE")
	}
	if tcp.CWR {
		flags = append(flags, "CWR")
	}
	if tcp.NS {
		flags = append(flags, "NS")
	}

	if len(flags) == 0 {
		return "NONE"
	}

	return strings.Join(flags, ",")
}

// getTCPFlagsValue 获取TCP flags的数值表示（可选，用于需要数值的场景）
func getTCPFlagsValue(tcp *layers.TCP) uint8 {
	var flags uint8

	if tcp.FIN {
		flags |= 0x01
	}
	if tcp.SYN {
		flags |= 0x02
	}
	if tcp.RST {
		flags |= 0x04
	}
	if tcp.PSH {
		flags |= 0x08
	}
	if tcp.ACK {
		flags |= 0x10
	}
	if tcp.URG {
		flags |= 0x20
	}
	if tcp.ECE {
		flags |= 0x40
	}
	if tcp.CWR {
		flags |= 0x80
	}

	return flags
}

func (nc *networkCapture) convert2Stream(frame *types.RawFrameData) {
	// 将tcp数据包转换成tcpStream，暂时不实现
}
