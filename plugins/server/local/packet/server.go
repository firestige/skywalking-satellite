package packet

import (
	"context"
	"fmt"

	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/capture"
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

}

type Server struct {
	// Configuration for the packet server
	config.CommonFields
	Afpacket AfpacketConfig `mapstructure:"afpacket"` // Configuration for afpacket capture
	Codec    CodecConfig    `mapstructure:"codec"`    // Configuration for packet codec

	// components
	dispatcherBuilder *handler.DispatcherBuilder
	pipeline          *Pipeline
	ctx               context.Context
	cancel            context.CancelFunc
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
# Network interface to capture packets on (default: any)
interface: "any"

# Ring buffer size for packet capture (default: 65536)
ring_size: 65536

# worker_count: Number of worker goroutines to process packets (default: 4)
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

# Ports to filter packets on (default: empty, captures all)
# Use YAML array syntax:
# ports: [5060, 8021]
ports:
  tcp: "38910 38914-38916"
  udp: "38917-38919 38911"
`
}

func (s *Server) GetServer() interface{} {
	return s
}

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

func (s *Server) Prepare() error {
	log.Logger.WithField("server", s.Name()).Info("packet server is preparing...")
	// Create context and wait group for pipeline lifecycle management
	s.ctx, s.cancel = context.WithCancel(context.Background())

	log.Logger.WithField("server", s.Name()).Info("packet server prepared successfully")
	return nil
}

func (s *Server) Start() error {
	log.Logger.WithField("server", s.Name()).Info("packet server is about to start...")
	// Build pipeline with configuration
	pipeline, err := s.buildPipeline(s.ctx)
	if err != nil {
		return fmt.Errorf("failed to build pipeline: %v", err)
	}
	s.pipeline = pipeline

	log.Logger.WithField("server", s.Name()).Info("packet server pipeline built successfully")

	// Prepare the pipeline
	if err := s.pipeline.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare pipeline: %v", err)
	}

	log.Logger.WithField("server", s.Name()).Info("packet server is starting...")

	// Start the pipeline
	if err := s.pipeline.Start(); err != nil {
		return fmt.Errorf("failed to start pipeline: %v", err)
	}

	log.Logger.WithField("server", s.Name()).Info("packet server started successfully")
	return nil
}

func (s *Server) Close() error {
	log.Logger.WithField("server", s.Name()).Info("packet server is closing...")

	// Cancel context to signal pipeline to stop
	if s.cancel != nil {
		s.cancel()
	}

	// Close the pipeline
	if s.pipeline != nil {
		if err := s.pipeline.Close(); err != nil {
			log.Logger.WithField("server", s.Name()).Errorf("failed to close pipeline: %v", err)
			return err
		}
	}

	log.Logger.WithField("server", s.Name()).Info("packet server closed")
	return nil
}

// buildPipeline creates and configures the pipeline based on server configuration
func (s *Server) buildPipeline(ctx context.Context) (*Pipeline, error) {

	dataSource, err := capture.NewNetworkCaptureBuilder(s.ctx).
		WithInterface(s.Afpacket.Interface).
		WithFilter(s.Afpacket.Filter).
		WithRingSize(s.Codec.RingSize).
		WithWorkerCount(s.Codec.WorkerCount).
		WithMTU(s.Codec.Mtu).
		WithLocalAddresses(s.Codec.LocalAddresses).
		Build()

	if err != nil {
		return nil, fmt.Errorf("failed to create data source: %v", err)
	}

	// Create frame handler/dispatcher
	dispatcher, err := s.dispatcherBuilder.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create frame handler: %v", err)
	}

	// Build pipeline using builder pattern
	pipeline, err := NewPipelineBuilder(dispatcher, ctx).
		WithSource(dataSource).
		Build()

	if err != nil {
		return nil, fmt.Errorf("failed to build pipeline: %v", err)
	}

	return pipeline, nil
}
