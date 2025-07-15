package packet

import (
	"context"
	"sync"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/utils"
	"github.com/google/gopacket"
	"github.com/google/gopacket/afpacket"
	"github.com/google/gopacket/layers"
)

type networkCapture struct {
	nic        string
	bpfFilter  string
	bufferSize int

	handle       *afpacket.TPacket // Handle for the AF_PACKET interface
	packetSource *gopacket.PacketSource
	closed       chan struct{} // Channel to signal closure
}

type NetworkCaptureBuilder struct {
	nic        string
	bpfFilter  string
	bufferSize int
}

func NewNetworkCaptureBuilder() *NetworkCaptureBuilder {
	return &NetworkCaptureBuilder{}
}

func (b *NetworkCaptureBuilder) WithInterface(iface string) *NetworkCaptureBuilder {
	b.nic = iface
	return b
}

func (b *NetworkCaptureBuilder) WithBPFFilter(filter string) *NetworkCaptureBuilder {
	b.bpfFilter = filter
	return b
}

func (b *NetworkCaptureBuilder) WithBufferSize(size int) *NetworkCaptureBuilder {
	b.bufferSize = size
	return b
}

func (b *NetworkCaptureBuilder) Build() (types.DataSource, error) {
	return &networkCapture{
		nic:        b.nic,
		bpfFilter:  b.bpfFilter,
		bufferSize: b.bufferSize,
		closed:     make(chan struct{}),
	}, nil
}

func (n *networkCapture) Fetch(ctx context.Context) (gopacket.Packet, error) {
	// Implement the logic to fetch raw packet data from the network interface
	return nil, nil
}

func (n *networkCapture) Prepare() error {
	log.Logger.Infof("Preparing network capture on interface: %s", n.nic)
	// Open the network interface for packet capture
	handle, err := afpacket.NewTPacket(
		afpacket.OptInterface(n.nic),
		afpacket.OptBlockSize(n.bufferSize),
		afpacket.TPacketVersion3,
	)
	if err != nil {
		log.Logger.Errorf("Failed to open AF_PACKET handle: %v", err)
		return err
	}

	bpfFilter, err := new(utils.PrebuildFilter).SIP(5060)
	if err != nil {
		log.Logger.Errorf("Failed to build BPF filter: %v", err)
		handle.Close()
		return err
	}

	handle.SetBPF(bpfFilter)

	n.handle = handle
	n.packetSource = gopacket.NewPacketSource(handle, layers.LinkTypeEthernet)
	return nil
}

func (n *networkCapture) Start(ctx context.Context, wg *sync.WaitGroup) error {
	// Implement the logic to start capturing packets
	// This could involve starting a goroutine that listens on the network interface
	// and processes incoming packets
	return nil
}

func (n *networkCapture) Close() error {
	// Implement the logic to clean up resources when stopping the capture
	// This could include closing network sockets, releasing buffers, etc.
	return nil
}
