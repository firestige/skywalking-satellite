package sip

import (
	"encoding/binary"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	sip "github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	packet "github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/sirupsen/logrus"
)

func (r *Receiver) processUDPFrame(frame *packet.RawFrameData) error {
	// 解析SIP消息
	// 由于可能存在gopacket不能正确识别SIP layer的情况，当gopacket无法解析时，直接使用GoSip解析UDP数据包
	p := frame.Packet
	data := r.extraSIPByGoPacket(p)
	if len(data) == 0 {
		data = r.getUdpPayload(p)
	}
	if len(data) == 0 {
		log.Logger.Debugf("No SIP data found in packet: %s", p)
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

func (r *Receiver) getUdpPayload(p gopacket.Packet) []byte {
	udpLayer := p.Layer(layers.LayerTypeUDP)
	if udpLayer == nil {
		return nil
	}
	data := udpLayer.LayerContents()
	if len(data) == 0 {
		return nil
	}
	if log.Logger.IsLevelEnabled(logrus.DebugLevel) {
		srcPort := binary.BigEndian.Uint16(data[0:2])
		dstPort := binary.BigEndian.Uint16(data[2:4])
		length := binary.BigEndian.Uint16(data[4:6])
		checksum := binary.BigEndian.Uint16(data[6:8])
		log.Logger.Debugf("UDP packet: srcPort=%d, dstPort=%d, length=%d, checksum=%d", srcPort, dstPort, length, checksum)
		log.Logger.Debugf("UDP packet actual has: %d(inclue frame header)", len(data))
	}
	return data[8:] // Skip UDP header (8 bytes)
}
