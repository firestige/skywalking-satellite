package sip

import (
	"bytes"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	sip "github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	packet "github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func (r *Receiver) processUDPFrame(frame *packet.RawFrameData) error {
	// 解析SIP消息
	// 由于可能存在gopacket不能正确识别SIP layer的情况，当gopacket无法解析时，直接使用GoSip解析UDP数据包
	p := frame.Packet
	data := r.extraSIPByGoPacket(p)
	if len(data) == 0 {
		var udp []byte
		data, udp = r.getUdpPayload(p)
		srcPort, dstPort := ParseUDPHeaderPorts(udp)
		frame.Connection.SrcPort = srcPort
		frame.Connection.DstPort = dstPort
	}
	if len(data) == 0 {
		log.Logger.Tracef("No SIP data found in packet: %s", p)
		return nil
	}
	goSipMsg, err := r.sipParser.Parse(data)
	if err != nil {
		// TODO bad packet, ignore and continue, need statistics
		return nil
	}

	// 转换消息
	var protocol string
	switch frame.Connection.Protocol {
	case layers.IPProtocolUDP:
		protocol = "UDP"
	case layers.IPProtocolTCP:
		protocol = "TCP"
	default:
		protocol = "Unknown"
	}
	var direction sip.Direction
	switch frame.Direction {
	case packet.Inbound:
		direction = sip.DirectionInbound
	case packet.Outbound:
		direction = sip.DirectionOutbound
	}

	conn := &sip.Connection{
		SrcIp:     frame.Connection.SrcHost,
		SrcPort:   frame.Connection.SrcPort,
		DstIp:     frame.Connection.DstHost,
		DstPort:   frame.Connection.DstPort,
		Protocol:  protocol,
		Direction: direction,
	}
	sipMsg := FromGoSip(goSipMsg, conn, frame.Timestamp)

	r.handler.HandleMessage(sipMsg)
	return nil
}

func (r *Receiver) attempToSkipUnwantedData(data []byte) ([]byte, int) {
	// 找到首行的CRLF
	if i := bytes.Index(data, []byte{'\r', '\n'}); i != -1 {
		log.Logger.Tracef("find first line: %s", data[:i])
		// 找到首行的左边界，左边界是首行的第一个可显大写字符，一般是SipMethod的第一个字符
		for j := 0; j < i; j++ {
			switch data[j] {
			case 'I', 'A', 'O', 'B', 'C', 'E', 'P', 'S', 'N', 'U', 'R', 'M':
				log.Logger.Tracef("skip %d bytes.", j)
				return data[j:], j // 返回首行之后的数据
			}
		}
	}
	return data, 0
}

func (r *Receiver) extraSIPByGoPacket(p gopacket.Packet) []byte {
	sipLayer := p.Layer(layers.LayerTypeSIP)
	if sipLayer == nil {
		return nil
	}
	data := sipLayer.LayerContents()
	if len(data) == 0 {
		return nil
	}
	return data
}

func (r *Receiver) getUdpPayload(p gopacket.Packet) ([]byte, []byte) {
	l := len(p.Data())
	log.Logger.Tracef("direct Extracting App payload from packet{len:%d}", l)
	var data []byte
	if layer := p.ApplicationLayer(); layer != nil {
		data = layer.Payload()
	}
	if len(data) == 0 {
		log.Logger.Tracef("No application layer data found in packet{len: %d}", l)
		if layer := p.TransportLayer(); layer != nil {
			data = layer.LayerPayload()
		}
	}
	if len(data) == 0 {
		log.Logger.Tracef("No transport layer data found in packet{len: %d}", l)
		if udpLayer := p.Layer(layers.LayerTypeUDP); udpLayer != nil {
			data = udpLayer.LayerPayload()
		}
	}
	if len(data) == 0 {
		log.Logger.Tracef("No UDP payload found in packet{len: %d}", l)
		data = p.Layer(layers.LayerTypeIPv4).LayerPayload()[8:] // Skip UDP header (8 bytes)
	}
	sipData, i := r.attempToSkipUnwantedData(data) // Skip UDP header (8 bytes)
	udpHeaderBytes := data[:i]
	return sipData, udpHeaderBytes
}

func ParseUDPHeaderPorts(udpHeaderBytes []byte) (srcPort, dstPort int) {
	if len(udpHeaderBytes) < 4 {
		return 0, 0 // 长度不足，无法解析
	}
	srcPort = int(udpHeaderBytes[0])<<8 | int(udpHeaderBytes[1])
	dstPort = int(udpHeaderBytes[2])<<8 | int(udpHeaderBytes[3])
	return srcPort, dstPort
}
