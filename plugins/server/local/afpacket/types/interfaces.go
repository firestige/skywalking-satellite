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

package types

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/google/gopacket"
)

// DataProcessor defines the function signature for processing SniffData
type DataProcessor func(*RawFrameData) error

// DataPipeline defines the interface for data pipeline
type DataPipeline interface {
	Prepare() error
	Start(ctx context.Context, wg *sync.WaitGroup) error
	Close() error
	Submit([]*RawFrameData) error
	GetStats() PipelineStats

	// SetDataProcessor sets the data processor function
	SetDataProcessor(name string, processor DataProcessor) error
	ClearDataProcessor() error
}

// PacketCapture defines the interface for packet capture
type PacketCapture interface {
	Prepare() error
	Start(ctx context.Context, wg *sync.WaitGroup) error
	Close() error
	GetPacketChannel() <-chan gopacket.Packet
	GetStats() CaptureStats
}

// HandlerManager defines the interface for managing packet handlers
type HandlerManager interface {
	Prepare() error
	Start(ctx context.Context, wg *sync.WaitGroup) error
	Close() error
	RegisterHandler(PacketHandler) error
	UnregisterHandler(name string) error
	GetHandlers() []PacketHandler
}

// EventLoop defines the interface for event loop
type EventLoop interface {
	Prepare() error
	Start(ctx context.Context, wg *sync.WaitGroup) error
	Close() error
	ProcessPacket(packet gopacket.Packet) error
}

// MonitoringManager handles monitoring and statistics
type MonitoringManager interface {
	io.Closer

	// Prepare initializes monitoring
	Prepare() error

	// Start begins monitoring
	Start(ctx context.Context, wg *sync.WaitGroup) error

	// ReportStats reports current statistics
	ReportStats() error

	// RecordDrop records a dropped packet
	RecordDrop(handlerName string, reason string)

	// RecordProcess records a processed packet
	RecordProcess(handlerName string, processingTime time.Duration)
}

// PacketHandler defines the interface for packet handlers
type PacketHandler interface {
	Handle(packet gopacket.Packet) ([]*RawFrameData, error)
	Type() string
	CanHandle(packet gopacket.Packet) bool
	Name() string
	Stats() HandlerStats
}

// HandlerStats contains statistics for packet handlers
type HandlerStats struct {
	PacketsHandled uint64
	DataGenerated  uint64
	ErrorCount     uint64
}

// CaptureStats contains statistics for packet capture
type CaptureStats struct {
	PacketsReceived uint64
	PacketsDropped  uint64
	BytesReceived   uint64
	ErrorCount      uint64
}

// PipelineStats contains statistics for data pipeline
type PipelineStats struct {
	ItemsProcessed uint64
	ItemsDropped   uint64
	QueueDepth     uint64
	ErrorCount     uint64
}

// ServerStats contains overall server statistics
type ServerStats struct {
	CaptureStats  CaptureStats
	PipelineStats PipelineStats
	HandlerStats  map[string]HandlerStats
}

// Config contains configuration for AFPacket server
type Config struct {
	Interface          string `mapstructure:"interface"`
	BufferSize         int    `mapstructure:"buffer_size"`
	Filter             string `mapstructure:"filter"`
	MaxWorkers         int    `mapstructure:"max_workers"`
	PipelineBufferSize int    `mapstructure:"pipeline_buffer_size"`
}

type RawFrameData struct {
	Protocol  string `mapstructure:"protocol"`
	Content   []byte `mapstructure:"content"`
	Timestamp int64  `mapstructure:"timestamp"`
}
