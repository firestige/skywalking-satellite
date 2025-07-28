package packet

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/capture"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/filter"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/handler"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/google/gopacket/layers"
	"github.com/sirupsen/logrus"
)

const (
	Name        = "packet-server"
	ShowName    = "Packet Server"
	Description = "A server plugin for packet capture and processing"
)

type AfpacketConfig struct {
	Interface    string `mapstructure:"interface"`     // Network interface to capture on
	SnapLen      int    `mapstructure:"snap_len"`      // Snapshot length for packet capture
	NumBlocks    int    `mapstructure:"num_blocks"`    // Number of blocks in the ring buffer
	BlockSize    int    `mapstructure:"block_size"`    // Size of each block in the ring buffer
	FlushTimeout int    `mapstructure:"flush_timeout"` // Timeout for flushing packets
	Filter       string `mapstructure:"filter"`        // BPF filter expression
}

type CodecConfig struct {
	// Codec configuration for packet processing
	RingSize       int      `mapstructure:"ring_size"`       // Size of the ring buffer for packet processing
	Mtu            int      `mapstructure:"mtu"`             // Maximum Transmission Unit for packet processing
	WorkerCount    int      `mapstructure:"worker_count"`    // Number of worker goroutines for processing packets
	LocalAddresses []string `mapstructure:"local_addresses"` // Local addresses to filter packets
	TCPChanSize    int      `mapstructure:"tcp_chan_size"`   // Size of the TCP channel for packet processing
	UDPChanSize    int      `mapstructure:"udp_chan_size"`   // Size of the UDP channel for packet processing
}

type Server struct {
	// Configuration for the packet server
	config.CommonFields
	Afpacket AfpacketConfig `mapstructure:"afpacket"` // Configuration for afpacket capture
	Codec    CodecConfig    `mapstructure:"codec"`    // Configuration for packet codec

	// Pipeline components integrated into server
	source            types.DataSource
	filterChain       types.FrameFilterChain
	dispatcherBuilder *handler.DispatcherBuilder
	dispatcher        types.FrameHandler

	// Lifecycle control
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
	mu      *sync.RWMutex
}

func (s *Server) Name() string {
	return Name
}

func (s *Server) ShowName() string {
	return ShowName
}

func (s *Server) Description() string {
	return Description
}

func (s *Server) DefaultConfig() string {
	return `
# Afpacket configuration
afpacket:
  # Network interface to capture packets on (default: any)
  interface: eth0
  # BPF filter expression (default: empty, captures all)
  filter: ""
  # Snapshot length for packet capture (default: 65536)
  snap_len: 65536
  # Number of blocks in the ring buffer (default: 128)
  num_blocks: 128
  # Size of each block in the ring buffer (default: 65536)
  block_size: 65536
  # Timeout for flushing packets in ms (default: 10)
  flush_timeout: 10

# Codec configuration
codec:
  # Ring buffer size for packet processing (default: 65536)
  ring_size: 65536
  # Number of worker goroutines to process packets (default: 4)
  worker_count: 4
  # MTU (Maximum Transmission Unit) size in bytes (default: 1500)
  mtu: 1500
  # Local addresses to capture packets from (default: empty, captures all)
  # Use YAML array syntax:
  # local_addresses: ["192.168.1.100", "10.0.0.1", "172.16.0.1"]
  # Or YAML list syntax:
  # local_addresses:
  #   - "192.168.1.100"
  #   - "10.0.0.1"
  #   - "172.16.0.1"
  local_addresses: []
  # Size of the TCP channel for packet processing (default: 1000)
  tcp_chan_size: 1000
  # Size of the UDP channel for packet processing (default: 1000)
  udp_chan_size: 1000
`
}

func (s *Server) GetServer() interface{} {
	return s
}

// RegisterHandler registers a packet handler for specific protocol and ports
func (s *Server) RegisterHandler(protocol layers.IPProtocol, ports string, name string, fn func(data *types.RawFrameData) error) {
	if s.dispatcherBuilder == nil {
		s.dispatcherBuilder = handler.NewDispatcherBuilder()
	}
	s.dispatcherBuilder.WithHandler(protocol, ports, name, fn)
	log.Logger.WithFields(logrus.Fields{
		"protocol": protocol,
		"ports":    ports,
		"name":     name,
	}).Info("Registered packet handler")
}

// Prepare initializes the server components but doesn't start packet capture
func (s *Server) Prepare() error {
	log.Logger.WithField("server", s.Name()).Info("Packet server is preparing...")

	// Create context for lifecycle management
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.mu = &sync.RWMutex{}

	log.Logger.WithField("server", s.Name()).Info("Packet server prepared successfully")
	return nil
}

func (s *Server) buildPipeline(ctx context.Context) error {
	var err error
	s.dispatcher, err = s.dispatcherBuilder.Build()
	if err != nil {
		log.Logger.WithField("server", s.Name()).Errorf("Failed to build dispatcher: %v", err)
		return err
	}
	s.filterChain = filter.NewFrameFilterChain(s.dispatcher, nil)
	processor := func(frame *types.RawFrameData) {
		s.filterChain.Filter(frame)
	}
	// TODO finish the builder
	s.source, err = capture.NewNetworkCaptureBuilder(ctx, processor).
		WithInterface(s.Afpacket.Interface).
		WithFilter(s.Afpacket.Filter).
		WithSnapLength(s.Afpacket.SnapLen).
		WithNumBlocks(s.Afpacket.NumBlocks).
		WithBlockSize(s.Afpacket.BlockSize).
		WithFlushTimeout(time.Duration(s.Afpacket.FlushTimeout) * time.Millisecond).
		WithTCPWorker(s.Codec.WorkerCount).
		WithUDPWorker(s.Codec.WorkerCount).
		WithTCPChanSize(s.Codec.TCPChanSize).
		WithUDPChanSize(s.Codec.UDPChanSize).
		Build()
	if err != nil {
		log.Logger.WithField("server", s.Name()).Errorf("Failed to create data source: %v", err)
	}
	return err
}

// Start begins packet capture and processing
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server is already running")
	}

	if err := s.buildPipeline(s.ctx); err != nil {
		log.Logger.WithField("server", s.Name()).Errorf("Failed to build pipeline: %v", err)
		return err
	}

	// Prepare data source
	if err := s.source.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare data source: %w", err)
	}

	log.Logger.WithField("server", s.Name()).Info("Packet server is starting...")

	s.source.Start()

	s.running = true
	log.Logger.WithField("server", s.Name()).Info("Packet server started successfully")
	return nil
}

// Close stops packet capture and cleans up resources
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	log.Logger.WithField("server", s.Name()).Info("Packet server is closing...")

	// Signal shutdown
	if s.cancel != nil {
		s.cancel()
	}

	// Close data source
	if s.source != nil {
		if err := s.source.Close(); err != nil {
			log.Logger.WithField("server", s.Name()).Errorf("Failed to close data source: %v", err)
			return err
		}
	}

	s.running = false
	log.Logger.WithField("server", s.Name()).Info("Packet server closed successfully")
	return nil
}

// IsRunning returns whether the server is currently running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetStats returns current processing statistics
func (s *Server) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"running": s.running,
		"server":  s.Name(),
	}

	// Add data source stats if available
	if s.source != nil {
		if statsProvider, ok := s.source.(interface{ GetStats() map[string]interface{} }); ok {
			sourceStats := statsProvider.GetStats()
			for k, v := range sourceStats {
				stats["source_"+k] = v
			}
		}
	}

	return stats
}
