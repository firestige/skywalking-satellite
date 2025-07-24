package capture

import (
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// TransportParser 传输层解析器（专注L2-L4层）
type TransportParser struct {
	options *Options
}

// NewTransportParser 创建传输层解析器
func NewTransportParser(options *Options) *TransportParser {
	return &TransportParser{
		options: options,
	}
}

// ParsedTransport 解析后的传输层数据
type ParsedTransport struct {
	// Raw packet
	Raw       gopacket.Packet
	Timestamp int64

	// 解析后的层信息
	Layers types.TransportLayers

	// 传输层信息
	TransportType types.TransportType
	Payload       []byte // 应用层载荷（不解析内容）
}

// Parse 解析网络包到传输层
func (tp *TransportParser) Parse(packet gopacket.Packet) *ParsedTransport {
	parsed := &ParsedTransport{
		Raw:       packet,
		Timestamp: packet.Metadata().Timestamp.UnixNano() / 1e6,
	}

	// 解析各层
	tp.parseDataLink(parsed)
	tp.parseNetwork(parsed)
	tp.parseTransport(parsed)

	return parsed
}

// parseDataLink 解析数据链路层
func (tp *TransportParser) parseDataLink(parsed *ParsedTransport) {
	if ethLayer := parsed.Raw.Layer(layers.LayerTypeEthernet); ethLayer != nil {
		if eth, ok := ethLayer.(*layers.Ethernet); ok {
			parsed.Layers.Ethernet = eth
		}
	}
}

// parseNetwork 解析网络层
func (tp *TransportParser) parseNetwork(parsed *ParsedTransport) {
	// IPv4
	if ipLayer := parsed.Raw.Layer(layers.LayerTypeIPv4); ipLayer != nil {
		if ip, ok := ipLayer.(*layers.IPv4); ok {
			parsed.Layers.IPv4 = ip
		}
	}

	// IPv6
	if ipLayer := parsed.Raw.Layer(layers.LayerTypeIPv6); ipLayer != nil {
		if ip, ok := ipLayer.(*layers.IPv6); ok {
			parsed.Layers.IPv6 = ip
		}
	}
}

// parseTransport 解析传输层
func (tp *TransportParser) parseTransport(parsed *ParsedTransport) {
	// TCP
	if tcpLayer := parsed.Raw.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		if tcp, ok := tcpLayer.(*layers.TCP); ok {
			parsed.Layers.TCP = tcp
			parsed.TransportType = types.TransportTCP
			parsed.Payload = tcp.Payload // 不解析应用层内容
			return
		}
	}

	// UDP
	if udpLayer := parsed.Raw.Layer(layers.LayerTypeUDP); udpLayer != nil {
		if udp, ok := udpLayer.(*layers.UDP); ok {
			parsed.Layers.UDP = udp
			parsed.TransportType = types.TransportUDP
			parsed.Payload = udp.Payload // 不解析应用层内容
			return
		}
	}

	// ICMP
	if icmpLayer := parsed.Raw.Layer(layers.LayerTypeICMPv4); icmpLayer != nil {
		if icmp, ok := icmpLayer.(*layers.ICMPv4); ok {
			parsed.Layers.ICMP = icmp
			parsed.TransportType = types.TransportICMP
			// ICMP 没有应用层载荷
			return
		}
	}

	parsed.TransportType = types.TransportUnknown
}

// GetTransportInfo 获取传输层连接信息
func (tp *TransportParser) GetTransportInfo(parsed *ParsedTransport) types.TransportInfo {
	info := types.TransportInfo{
		Type: parsed.TransportType.String(),
	}

	// 设置IP地址
	if parsed.Layers.IPv4 != nil {
		info.SrcIP = parsed.Layers.IPv4.SrcIP.String()
		info.DstIP = parsed.Layers.IPv4.DstIP.String()
		info.IPVersion = 4
	} else if parsed.Layers.IPv6 != nil {
		info.SrcIP = parsed.Layers.IPv6.SrcIP.String()
		info.DstIP = parsed.Layers.IPv6.DstIP.String()
		info.IPVersion = 6
	}

	// 设置端口（仅TCP/UDP有端口概念）
	switch parsed.TransportType {
	case types.TransportTCP:
		info.SrcPort = int(parsed.Layers.TCP.SrcPort)
		info.DstPort = int(parsed.Layers.TCP.DstPort)
	case types.TransportUDP:
		info.SrcPort = int(parsed.Layers.UDP.SrcPort)
		info.DstPort = int(parsed.Layers.UDP.DstPort)
	}

	// 判断方向
	info.Direction = tp.determineDirection(info.SrcIP, info.DstIP)

	return info
}

// determineDirection 基于IP判断数据方向
func (tp *TransportParser) determineDirection(srcIP, dstIP string) string {
	for _, localAddr := range tp.options.LocalAddresses {
		if srcIP == localAddr {
			return "outbound"
		}
		if dstIP == localAddr {
			return "inbound"
		}
	}
	return "inbound"
}
