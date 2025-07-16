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
	"github.com/google/gopacket/pcap"
	"golang.org/x/net/bpf"
)

type networkCapture struct {
	handle                 *afpacket.TPacket // Handle for the AF_PACKET interface
	ringBuffer             *utils.RingBuffer
	tcpAssembler           *TCPAssembler
	lengthFieldBaseDecoder *LengthFieldBaseDecoder

	packetPool sync.Pool
	bufferPool sync.Pool

	config *CaptureConfig

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	nic        string
	bpfFilter  string
	bufferSize int

	packetSource *gopacket.PacketSource
	closed       chan struct{} // Channel to signal closure
}

type CaptureConfig struct {
	Interface   string
	BPFFilter   []bpf.RawInstruction
	SnapLen     int
	BufferSize  int
	RingSize    int
	WorkerCount int
	MTU         int
}

func newNetworkCapture(config *CaptureConfig, ctx context.Context) (types.DataSource, error) {
	handle, err := afpacket.NewTPacket(
		afpacket.OptInterface(config.Interface),
		afpacket.OptSnapLen(config.SnapLen),
		afpacket.OptNumBlocks(128),
		afpacket.OptBlockSize(1204*1024),
		afpacket.OptPollTimeout(pcap.BlockForever),
		afpacket.TPacketVersion3,
	)
	if err != nil {
		log.Logger.Errorf("Failed to create AF_PACKET handle: %v", err)
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	nc := &networkCapture{
		handle:                 handle,
		ringBuffer:             utils.NewRingBuffer(config.RingSize),
		tcpAssembler:           NewTCPAssembler(),
		lengthFieldBaseDecoder: &LengthFieldBaseDecoder{},
		config:                 config,
		ctx:                    ctx,
		cancel:                 cancel,
	}

	nc.initPools()

	return nc, nil
}

func (n *networkCapture) initPools() {
	n.packetPool = sync.Pool{
		New: func() interface{} {
			return &utils.PacketInfo{}
		},
	}

	n.bufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, n.config.MTU)
		},
	}
}

func (n *networkCapture) Fetch(ctx context.Context) (types.RawFrameData, error) {
	log.Logger.Info("Fetching packet from network interface")
	defer log.Logger.Info("Fetch stopped")
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

	bpfFilter, err := utils.PrebuildFilter.SIP(5060)
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
	log.Logger.Infof("Closing network capture on interface: %s", n.nic)
	close(n.closed)
	if n.handle != nil {
		n.handle.Close()
		n.handle = nil
		n.packetSource = nil
	}
	log.Logger.Info("Network capture closed successfully")
	return nil
}

type NetworkCaptureBuilder struct {
	config *CaptureConfig
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
