package packet

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/capture"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/filter"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
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
	receiverMapping map[string]map[string]func(*types.RawFrameData) error // Mapping of protocol to handler
	source          types.DataSource
	filterChain     types.FrameFilterChain
	dispatcher      types.FrameHandler

	// 并发控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup
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
	return ``
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

	// Create context and wait group for lifecycle management
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.wg = &sync.WaitGroup{}

	// Build components
	if err := s.buildComponents(); err != nil {
		return fmt.Errorf("failed to build components: %v", err)
	}

	// Prepare data source
	if err := s.source.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare source: %v", err)
	}

	log.Logger.WithField("server", s.Name()).Info("packet server prepared successfully")
	return nil
}

func (s *Server) Start() error {
	log.Logger.WithField("server", s.Name()).Info("packet server is starting...")

	// Start data source
	if err := s.source.Start(); err != nil {
		return fmt.Errorf("failed to start source: %v", err)
	}

	// Start processing loop
	s.wg.Add(1)
	go s.run()

	log.Logger.WithField("server", s.Name()).Info("packet server started successfully")
	return nil
}

func (s *Server) Close() error {
	log.Logger.WithField("server", s.Name()).Info("packet server is closing...")

	// Cancel context to signal goroutines to stop
	if s.cancel != nil {
		s.cancel()
	}

	// Wait for all goroutines to finish
	if s.wg != nil {
		s.wg.Wait()
	}

	// Close data source
	if s.source != nil {
		if err := s.source.Close(); err != nil {
			log.Logger.WithField("server", s.Name()).WithError(err).Error("failed to close source")
		}
	}

	log.Logger.WithField("server", s.Name()).Info("packet server closed")
	return nil
}

// buildComponents
func (s *Server) buildComponents() error {
	// Create data source
	dataSource, err := capture.NewNetworkCaptureBuilder(s.ctx).
		WithInterface(s.Afpacket.Interface).
		WithFilter(s.Afpacket.Filter).
		WithRingSize(s.Codec.RingSize).
		WithWorkerCount(s.Codec.WorkerCount).
		WithMTU(s.Codec.Mtu).
		WithLocalAddresses(s.Codec.LocalAddresses).
		Build()

	if err != nil {
		return fmt.Errorf("failed to create data source: %v", err)
	}
	s.source = dataSource

	// Create frame handler/dispatcher
	builder := NewDispatcherBuilder()
	for protocol, handlers := range s.receiverMapping {
		for name, handler := range handlers {
			builder.WithHandler(protocol, name, handler)
		}
	}
	dispatcher, err := builder.Build()
	if err != nil {
		return fmt.Errorf("failed to create frame handler: %v", err)
	}
	s.dispatcher = dispatcher

	// Create filter chain (如果需要过滤器的话)
	s.filterChain = filter.NewFrameFilterChain(s.dispatcher, []types.FrameFilter{})

	return nil
}

// run 处理循环 (原来pipeline的run方法)
func (s *Server) run() {
	defer func() {
		s.wg.Done()
		if r := recover(); r != nil {
			log.Logger.WithFields(logrus.Fields{
				"server": s.Name(),
				"error":  r,
				"stack":  string(debug.Stack()),
			}).Error("Server processing panic recovered")
		}
	}()

	log.Logger.WithField("server", s.Name()).Info("Server processing loop started")

	for {
		select {
		case <-s.ctx.Done():
			log.Logger.WithField("server", s.Name()).Info("Server context done, stopping processing loop...")
			return
		default:
			frame, err := s.source.Fetch()
			if err != nil {
				log.Logger.WithField("server", s.Name()).WithError(err).Error("Error fetching frame")
				continue
			}

			if frame == types.EmptyRawFrameData {
				continue
			}

			// 通过过滤链处理frame
			s.filterChain.Filter(frame)
		}
	}
}
