package capture

import (
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/google/gopacket"
	"github.com/google/gopacket/afpacket"
	"github.com/google/gopacket/layers"
	"github.com/sirupsen/logrus"
)

// captureLoop 主抓包循环
func (nc *networkCapture) captureLoop() {
	defer nc.wg.Done()

	handle, err := nc.createPacketHandle()
	if err != nil {
		log.Logger.Errorf("Failed to create packet handle: %v", err)
		return
	}
	defer handle.Close()
	log.Logger.Infof("Packet handle created for interface %s", nc.config.Interface)
	nc.handle = handle

	packetSource := gopacket.NewPacketSource(nc.handle, layers.LinkTypeEthernet)
	packetSource.DecodeOptions.Lazy = true
	packetSource.DecodeOptions.NoCopy = true

	for {
		select {
		case <-nc.ctx.Done():
			log.Logger.Info("Capture loop stopping...")
			return
		case packet, ok := <-packetSource.Packets():
			if !ok {
				log.Logger.Info("Packet source closed")
				return
			}
			if log.Logger.IsLevelEnabled(logrus.TraceLevel) {
				log.Logger.Tracef("Captured packet: %s", packet)
			} else if log.Logger.IsLevelEnabled(logrus.DebugLevel) {
				log.Logger.Debugf("Captured packet: len=%d, src=%s, dst=%s, layers=%d", len(packet.Data()), packet.NetworkLayer().NetworkFlow().Src().String(), packet.NetworkLayer().NetworkFlow().Dst().String(), len(packet.Layers()))
			}
			if packet == nil {
				continue
			}
			nc.dispatchPacket(packet)
		}
	}
}

func (nc *networkCapture) dispatchPacket(packet gopacket.Packet) {
	frame := &types.RawFrameData{}
	parseTransportLayers(packet, frame)
	nc.refreshDirection(frame)
	protocol := frame.Connection.Protocol

	switch protocol {
	case layers.IPProtocolTCP:
		nc.dispatchTCPPacket(frame)
	case layers.IPProtocolUDP:
		nc.dispatchUDPPacket(frame)
	default:
		log.Logger.Debugf("Unsupported IP protocol: %s", protocol)
		// todo add drop packet statistics
		return
	}
}

func (nc *networkCapture) refreshDirection(frame *types.RawFrameData) {
	if frame.Direction == types.Unknown {
		srcIP := frame.Connection.SrcHost
		dstIP := frame.Connection.DstHost
		if srcIP == nc.config.LocalIP {
			frame.Direction = types.Outbound
			return
		}
		if dstIP == nc.config.LocalIP {
			frame.Direction = types.Inbound
			return
		}
		log.Logger.Tracef("Cannot determine direction for frame: {src:%s:%d, dst:%s:%d}", srcIP, frame.Connection.SrcPort, dstIP, frame.Connection.DstPort)
	}
}

func (nc *networkCapture) dispatchUDPPacket(frame *types.RawFrameData) {
	select {
	case <-nc.ctx.Done():
		return
	case nc.udpChan <- frame:
		return
	default:
		log.Logger.Warn("UDP channel is full, dropping frame")
		// TODO add drop packet statistics
		return
	}
}

func (nc *networkCapture) dispatchTCPPacket(frame *types.RawFrameData) {
	select {
	case <-nc.ctx.Done():
		return
	case nc.tcpChan <- frame:
		return
	default:
		log.Logger.Warn("TCP channel is full, dropping frame")
		// todo add drop packet statistics
		return
	}
}

func (nc *networkCapture) createPacketHandle() (*afpacket.TPacket, error) {
	handle, err := afpacket.NewTPacket(
		afpacket.OptInterface(nc.config.Interface),
		afpacket.OptFrameSize(nc.config.SnapLen),
		afpacket.OptNumBlocks(nc.config.NumBlocks),
		afpacket.OptBlockSize(nc.config.BlockSize),
		afpacket.OptPollTimeout(nc.config.FlushTimeout),
		afpacket.TPacketVersion3,
	)
	if err != nil {
		log.Logger.Errorf("Failed to create AF_PACKET handle: %v", err)
		return nil, err
	}
	if err := handle.SetBPF(nc.config.Filter); err != nil {
		log.Logger.Errorf("Failed to set BPF filter: %v", err)
		handle.Close()
		return nil, err
	}
	log.Logger.Infof("Created AF_PACKET handle for interface %s", nc.config.Interface)
	return handle, nil
}

func (nc *networkCapture) tcpProcessLoop() {
	defer nc.wg.Done()

	for {
		select {
		case <-nc.ctx.Done():
			log.Logger.Info("TCP process loop stopping...")
			return
		case frame, ok := <-nc.tcpChan:
			if !ok {
				log.Logger.Info("TCP channel closed")
				return
			}
			nc.convert2Stream(frame)
		}
	}
}

func (nc *networkCapture) udpProcessLoop() {
	defer nc.wg.Done()

	for {
		select {
		case <-nc.ctx.Done():
			log.Logger.Info("UDP process loop stopping...")
			return
		case frame, ok := <-nc.udpChan:
			if !ok {
				log.Logger.Info("UDP channel closed")
				return
			}
			nc.frameConsumer(frame) // 直接将UDP帧传递给处理器
		}
	}
}
