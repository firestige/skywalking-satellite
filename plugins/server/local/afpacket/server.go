// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Apache Software Foundation (ASF) licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package afpacket

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/afpacket/types"
)

const (
	Name     = "afpacket-server"
	ShowName = "AFPacket Server"
)

// Server implements the SkyWalking Satellite Server interface for packet capture
type Server struct {
	config.CommonFields

	// Configuration
	Interface     string        `mapstructure:"interface"`      // Network interface to capture on
	BufferSize    int           `mapstructure:"buffer_size"`    // Ring buffer size
	Filter        string        `mapstructure:"filter"`         // BPF filter expression
	StatsInterval time.Duration `mapstructure:"stats_interval"` // Statistics reporting interval
	DropThreshold int           `mapstructure:"drop_threshold"` // Drop count threshold for reporting

	// Core components
	capture    types.PacketCapture
	eventLoop  types.EventLoop
	handlerMgr types.HandlerManager
	pipeline   types.DataPipeline
	monitoring types.MonitoringManager

	// Lifecycle management
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started bool
	mu      sync.RWMutex
}

func (s *Server) Name() string {
	return Name
}

func (s *Server) ShowName() string {
	return ShowName
}

func (s *Server) Description() string {
	return "AFPacket server for network packet capture using AF_PACKET v3"
}

func (s *Server) DefaultConfig() string {
	return `
# Network interface to capture packets on
interface: "eth0"
# Ring buffer size (number of blocks)
buffer_size: 1024
# BPF filter expression for packet filtering
filter: "tcp or udp"
# Statistics reporting interval
stats_interval: 10s
# Drop count threshold for reporting alerts
drop_threshold: 100
# Handler configurations
handlers:
  - name: "http-handler"
    enabled: true
    pipeline_buffer_size: 1000
    drop_policy: "drop_oldest"
  - name: "tcp-handler"
    enabled: true
    pipeline_buffer_size: 500
    drop_policy: "block"
`
}

func (s *Server) Prepare() error {
	log.Logger.Info("afpacket server is preparing...")

	// Initialize context for lifecycle management
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Initialize components
	if err := s.initComponents(); err != nil {
		return fmt.Errorf("failed to initialize components: %w", err)
	}

	// Prepare all components
	if err := s.prepareComponents(); err != nil {
		return fmt.Errorf("failed to prepare components: %w", err)
	}

	log.Logger.Info("afpacket server prepared successfully")
	return nil
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("afpacket server is already started")
	}

	log.Logger.Info("starting afpacket server...")

	// Start components in order
	if err := s.startComponents(); err != nil {
		return fmt.Errorf("failed to start components: %w", err)
	}

	s.started = true
	log.Logger.Info("afpacket server started successfully")
	return nil
}

func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	log.Logger.Info("closing afpacket server...")

	// Signal all components to stop
	s.cancel()

	// Close components in reverse order
	s.closeComponents()

	// Wait for all goroutines to finish
	s.wg.Wait()

	s.started = false
	log.Logger.Info("afpacket server closed successfully")
	return nil
}

func (s *Server) GetServer() interface{} {
	return s
}

func (s *Server) RegisterHandler(protocol string, handler func(*types.RawFrameData) error) error {
	return s.pipeline.SetDataProcessor(protocol, handler)
}

// Initialize all components
func (s *Server) initComponents() error {
	// Initialize packet capture
	s.capture = NewPacketCapture(s.Interface, s.BufferSize, s.Filter)

	// Initialize handler manager
	s.handlerMgr = NewHandlerManager()

	// Initialize data pipeline
	s.pipeline = NewDataPipeline()

	// Initialize event loop
	s.eventLoop = NewEventLoop(s.capture, s.handlerMgr, s.pipeline)

	// Initialize monitoring
	s.monitoring = NewMonitoringManager(s.StatsInterval, s.DropThreshold)

	return nil
}

// Prepare all components
func (s *Server) prepareComponents() error {
	// Prepare packet capture
	if err := s.capture.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare packet capture: %w", err)
	}

	// Prepare handler manager
	if err := s.handlerMgr.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare handler manager: %w", err)
	}

	// Prepare data pipeline
	if err := s.pipeline.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare data pipeline: %w", err)
	}

	// Prepare event loop
	if err := s.eventLoop.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare event loop: %w", err)
	}

	// Prepare monitoring
	if err := s.monitoring.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare monitoring: %w", err)
	}

	return nil
}

// Start all components
func (s *Server) startComponents() error {
	// Start monitoring first
	if err := s.monitoring.Start(s.ctx, &s.wg); err != nil {
		return fmt.Errorf("failed to start monitoring: %w", err)
	}

	// Start data pipeline
	if err := s.pipeline.Start(s.ctx, &s.wg); err != nil {
		return fmt.Errorf("failed to start data pipeline: %w", err)
	}

	// Start handler manager
	if err := s.handlerMgr.Start(s.ctx, &s.wg); err != nil {
		return fmt.Errorf("failed to start handler manager: %w", err)
	}

	// Start event loop
	if err := s.eventLoop.Start(s.ctx, &s.wg); err != nil {
		return fmt.Errorf("failed to start event loop: %w", err)
	}

	// Start packet capture last
	if err := s.capture.Start(s.ctx, &s.wg); err != nil {
		return fmt.Errorf("failed to start packet capture: %w", err)
	}

	return nil
}

// Close all components in reverse order
func (s *Server) closeComponents() {
	// Close components in reverse order of startup
	if s.capture != nil {
		s.capture.Close()
	}

	if s.eventLoop != nil {
		s.eventLoop.Close()
	}

	if s.handlerMgr != nil {
		s.handlerMgr.Close()
	}

	if s.pipeline != nil {
		s.pipeline.Close()
	}

	if s.monitoring != nil {
		s.monitoring.Close()
	}
}
