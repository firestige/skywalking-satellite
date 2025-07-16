package packet

import (
	"context"
	"fmt"
	"sync"

	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/api"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/utils"
)

const (
	Name        = "packet-server"
	ShowName    = "Packet Server"
	Description = "A server plugin for packet capture and processing"
)

type serverAdapter struct {
	config.CommonFields

	Interface      string   `mapstructure:"interface"`       // Network interface to capture on
	BufferSize     int      `mapstructure:"buffer_size"`     // Ring buffer size
	RingSize       int      `mapstructure:"ring_size"`       // Ring buffer size
	WorkerCount    int      `mapstructure:"worker_count"`    // Number of worker goroutines
	MTU            int      `mapstructure:"mtu"`             // Maximum Transmission Unit
	LocalAddresses []string `mapstructure:"local_addresses"` // Local addresses to filter

	pipeline *Pipeline
	ctx      context.Context
	cancel   context.CancelFunc
	wg       *sync.WaitGroup
}

func NewServer() api.Server {
	return &serverAdapter{}
}

func (s *serverAdapter) Name() string {
	return Name
}

func (s *serverAdapter) ShowName() string {
	return ShowName
}

func (s *serverAdapter) Description() string {
	return Description
}

func (s *serverAdapter) DefaultConfig() string {
	return `
# Network interface to capture packets on (default: any)
interface: "any"

# Ring buffer size for packet capture (default: 65536)
buffer_size: 65536

# BPF filter expression for packet filtering (default: empty, captures all)
filter: ""

# Statistics reporting interval in seconds (default: 10)
stats_interval: 10

# Drop count threshold for reporting warnings (default: 1000)
drop_threshold: 1000
`
}

func (s *serverAdapter) GetServer() interface{} {
	return s
}

func (s *serverAdapter) Prepare() error {
	log.Logger.WithField("server", s.Name()).Info("packet server is preparing...")

	// Set default values if not configured
	if s.Interface == "" {
		s.Interface = "any"
	}
	if s.BufferSize <= 0 {
		s.BufferSize = 65536
	}

	// Build pipeline with configuration
	pipeline, err := s.buildPipeline()
	if err != nil {
		return fmt.Errorf("failed to build pipeline: %v", err)
	}
	s.pipeline = pipeline

	// Prepare the pipeline
	if err := s.pipeline.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare pipeline: %v", err)
	}

	log.Logger.WithField("server", s.Name()).Info("packet server prepared successfully")
	return nil
}

func (s *serverAdapter) Start() error {
	log.Logger.WithField("server", s.Name()).Info("packet server is starting...")

	// Create context and wait group for pipeline lifecycle management
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.wg = &sync.WaitGroup{}

	// Start the pipeline
	if err := s.pipeline.Start(s.ctx, s.wg); err != nil {
		return fmt.Errorf("failed to start pipeline: %v", err)
	}

	log.Logger.WithField("server", s.Name()).Info("packet server started successfully")
	return nil
}

func (s *serverAdapter) Close() error {
	log.Logger.WithField("server", s.Name()).Info("packet server is closing...")

	// Cancel context to signal pipeline to stop
	if s.cancel != nil {
		s.cancel()
	}

	// Wait for pipeline to finish
	if s.wg != nil {
		s.wg.Wait()
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
func (s *serverAdapter) buildPipeline() (*Pipeline, error) {
	// Create data source with configuration
	bpfFilter, err := utils.NewFilterBuilder().IPv4OrDrop().TCP(utils.JumpToIfNoMatch("check_udp")).PortOrAccept(5060, "check_udp").UDP(utils.WithLabel("check_udp").OrDrop()).PortOrDrop(5060).Compile()
	if err != nil {
		return nil, fmt.Errorf("failed to compile BPF filter: %v", err)
	}

	dataSource, err := NewNetworkCaptureBuilder().
		WithInterface(s.Interface).
		WithBPFFilter(bpfFilter).
		WithBufferSize(s.BufferSize).
		WithRingSize(s.RingSize).
		WithWorkerCount(s.WorkerCount).
		WithMTU(s.MTU).
		WithLocalAddresses(s.LocalAddresses).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create data source: %v", err)
	}

	// Create frame handler/dispatcher
	dispatcher, err := NewDispatcherBuilder().WithHandler(types.SIP, "", nil).WithHandler(types.ESL, "", nil).Build()
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
