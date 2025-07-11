package afpacket

import (
	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/google/gopacket"
	"github.com/google/gopacket/afpacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"golang.org/x/net/bpf"
)

const (
	// Name is the name of the AFPacket server.
	Name     = "afpacket-server"
	ShowName = "AFPacket Server"
)

type Server struct {
	config.CommonFields
	// NicName is the network interface name to listen on, e.g., "eth0".
	NicName   string `mapstructure:"nic_name"`
	FrameSize int    `mapstructure:"frame_size"` // Frame size for packet capture, default is 65536 bytes.
	BpfFilter string `mapstructure:"bpf_filter"` // BPF filter to capture specific types of packets, e.g., "tcp or udp or icmp or icmp6 or ip or ipv6"
	// Handle is the afpacket handle for capturing packets.
	EventLoop *EventLoop
	// OutputChannel is the channel where captured packets will be sent.
}

func (s *Server) Name() string {
	return Name
}

func (s *Server) ShowName() string {
	return ShowName
}

func (s *Server) Description() string {
	return "AFPacket Server captures network packets from a specified interface and processes them for analysis."
}

func (s *Server) DefaultConfig() string {
	return `
# nic_name is the network interface to capture traffic from
nic_name: eth0  # The network interface to capture traffic from
# frame_size is the size of the frame to capture, default is 65536 bytes.
frame_size: 65536  # The size of the frame to capture, default is 65536 bytes.
# You can set a BPF filter to capture specific types of packets, e.g., "
# tcp or udp or icmp or icmp6 or ip or ipv6"
bpf_filter: "tcp or udp"  # BPF filter to capture specific types of packets
`
}

func (s *Server) Prepare() error {
	s.EventLoop = NewEventLoop(1024)
	return nil
}

func (s *Server) Start() error {
	log.Logger.WithField("nic_name", s.NicName).Info("AFPacket server is starting...")
	go func() {
		handle, err := afpacket.NewTPacket(
			afpacket.OptInterface(s.NicName),
			afpacket.OptFrameSize(65536),
			afpacket.TPacketVersion3,
		)
		if err != nil {
			log.Logger.Errorf("Error creating af_packet handle: %v", err)
			return
		}

		rawIns, err := pcap.CompileBPFFilter(layers.LinkTypeEthernet, 65535, s.BpfFilter)
		if err != nil {
			log.Logger.Fatalf("Error compiling BPF filter string: %v", err)
		}
		bpfIns := make([]bpf.RawInstruction, len(rawIns))
		for i, ins := range rawIns {
			bpfIns[i] = bpf.RawInstruction{
				Op: ins.Code,
				Jt: ins.Jt,
				Jf: ins.Jf,
				K:  ins.K,
			}
		}
		// handle.SetBPF(bpfIns) // Set BPF filter to capture specific types of packets

		// handle.SetBPF("tcp or udp or icmp or icmp6 or ip or ipv6") // Set BPF filter to capture TCP, UDP, ICMP, and IPv6 packets
		defer func() {
			handle.Close()
			log.Logger.Info("AFPacket handle closed")
		}()
		packetSource := gopacket.NewPacketSource(handle, layers.LinkTypeEthernet)
		for packet := range packetSource.Packets() {
			if sip := packet.Layer(layers.LayerTypeSIP); sip != nil {
				content := sip.LayerContents()
				event := &Event{
					Name:      "afpacket",
					Timestamp: packet.Metadata().CaptureInfo.Timestamp.UnixNano() / 1e6, // Convert to milliseconds
					Payload:   content,
				}
				// Send the event to the event loop
				s.EventLoop.Publish(event)
			}
			// Process the packet
			// if app := packet.ApplicationLayer(); app != nil {
			// 	payload := app.Payload()
			// 	l := len(payload)
			// 	if l > 0 {
			// 		log.Logger.Printf("Application Layer Payload (%d bytes): %s", l, payload)
			// 	} else {
			// 		// log.Logger.Warn("Application Layer Payload is empty")
			// 		continue
			// 	}
			// 	event := &Event{
			// 		Name:      "afpacket",
			// 		Timestamp: packet.Metadata().CaptureInfo.Timestamp.UnixNano() / 1e6, // Convert to milliseconds
			// 		Payload:   payload,
			// 	}
			// 	// Send the event to the event loop
			// 	s.EventLoop.Publish(event)
			// }
		}
	}()
	s.EventLoop.Start()
	log.Logger.Info("AFPacket server started successfully")
	return nil
}

func (s *Server) Close() error {
	log.Logger.Info("AFPacket server is closed")
	if s.EventLoop != nil {
		s.EventLoop.Stop()
		log.Logger.Info("Event loop stopped")
	} else {
		log.Logger.Warn("Event loop is nil, nothing to stop")
	}
	return nil
}

func (s *Server) GetServer() interface{} {
	return s
}
