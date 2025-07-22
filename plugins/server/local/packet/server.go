package packet

import (
	"context"
	"fmt"

	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/utils"
	"github.com/sirupsen/logrus"
)

const (
	Name        = "packet-server"
	ShowName    = "Packet Server"
	Description = "A server plugin for packet capture and processing"
)

type Server struct {
	config.CommonFields

	Interface      string   `mapstructure:"interface"`       // Network interface to capture on
	RingSize       int      `mapstructure:"ring_size"`       // Ring buffer size
	WorkerCount    int      `mapstructure:"worker_count"`    // Number of worker goroutines
	MTU            int      `mapstructure:"mtu"`             // Maximum Transmission Unit
	LocalAddresses []string `mapstructure:"local_addresses"` // Local addresses to filter
	Ports          []int    `mapstructure:"ports"`           // Ports to filter

	receiverMapping map[string]map[string]func(*types.RawFrameData) error // Mapping of protocol to handler
	pipeline        *Pipeline
	ctx             context.Context
	cancel          context.CancelFunc
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
ports: [5060, 8021]
`
}

func (s *Server) GetServer() interface{} {
	return s
}

func (s *Server) RegisterHandler(protocol string, name string, handler func(data *types.RawFrameData) error) {
	if s.receiverMapping == nil {
		s.receiverMapping = make(map[string]map[string]func(*types.RawFrameData) error)
	}
	if _, exists := s.receiverMapping[protocol]; !exists {
		s.receiverMapping[protocol] = make(map[string]func(*types.RawFrameData) error)
	}
	if _, exists := s.receiverMapping[protocol][name]; exists {
		log.Logger.WithFields(logrus.Fields{
			"protocol": protocol,
			"name":     name,
		}).Warn("Handler already exists, overwriting")
	}
	s.receiverMapping[protocol][name] = handler
	log.Logger.WithFields(logrus.Fields{
		"protocol": protocol,
		"name":     name,
	}).Info("Registered packet handler")
}

func (s *Server) Prepare() error {
	log.Logger.WithField("server", s.Name()).Info("packet server is preparing...")

	// Set default values if not configured
	if s.Interface == "" {
		s.Interface = "any"
	}

	// Create context and wait group for pipeline lifecycle management
	s.ctx, s.cancel = context.WithCancel(context.Background())

	log.Logger.WithField("server", s.Name()).Info("packet server prepared successfully")
	return nil
}

func (s *Server) Start() error {
	log.Logger.WithField("server", s.Name()).Info("packet server is about to start...")
	// Build pipeline with configuration
	pipeline, err := s.buildPipeline()
	if err != nil {
		return fmt.Errorf("failed to build pipeline: %v", err)
	}
	s.pipeline = pipeline

	// Prepare the pipeline
	if err := s.pipeline.Prepare(s.ctx); err != nil {
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
func (s *Server) buildPipeline() (*Pipeline, error) {
	// Create data source with configuration
	// Convert s.Ports from []int to []uint32
	var portsUint32 []uint32
	for _, p := range s.Ports {
		portsUint32 = append(portsUint32, uint32(p))
	}

	bpfFilter, err := utils.NewFilterBuilder().
		IPv4OrDrop().
		TCP(utils.JumpToIfNoMatch("check_udp")).
		PortsOrAccept(portsUint32, "check_udp").
		UDP(utils.WithLabel("check_udp").OrDrop()).
		PortsOrDrop(portsUint32).
		Accept().
		Drop().
		Compile()

	if err != nil {
		return nil, fmt.Errorf("failed to compile BPF filter: %v", err)
	}

	dataSource, err := NewNetworkCaptureBuilder().
		WithInterface(s.Interface).
		WithBPFFilter(bpfFilter).
		WithRingSize(s.RingSize).
		WithWorkerCount(s.WorkerCount).
		WithMTU(s.MTU).
		WithLocalAddresses(s.LocalAddresses).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create data source: %v", err)
	}

	// Create frame handler/dispatcher
	builder := NewDispatcherBuilder()
	for protocol, handlers := range s.receiverMapping {
		for name, handler := range handlers {
			builder.WithHandler(protocol, name, handler)
		}
	}
	dispatcher, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create frame handler: %v", err)
	}

	// Build pipeline using builder pattern
	pipeline, err := NewPipelineBuilder(dispatcher).
		WithSource(dataSource).
		Build()

	if err != nil {
		return nil, fmt.Errorf("failed to build pipeline: %v", err)
	}

	return pipeline, nil
}
