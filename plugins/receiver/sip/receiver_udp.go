package sip

import (
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	sip "github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	packet "github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/google/gopacket/layers"
)

func (r *Receiver) processUDPFrame(frame *packet.RawFrameData) error {
	// 解析SIP消息
	p := frame.Packet
	sipLayer := p.Layer(layers.LayerTypeSIP)
	if sipLayer == nil {
		log.Logger.Debug("No SIP layer found in packet, ignore")
		return nil
	}
	data := sipLayer.LayerContents()
	goSipMsg, err := r.sipParser.Parse(data)
	if err != nil {
		log.Logger.WithError(err).Debugf("failed to parse SIP message: %s", data)
		// bad packet, ignore and continue, need statistics
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
