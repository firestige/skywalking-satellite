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
	"io"
	"sync"
	"time"

	"github.com/google/gopacket"
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"
)

// PacketCapture handles low-level packet capture using AF_PACKET v3
type PacketCapture interface {
	io.Closer

	// Prepare initializes the packet capture
	Prepare() error

	// Start begins packet capture
	Start(ctx context.Context, wg *sync.WaitGroup) error

	// GetPacketSource returns the gopacket source for reading packets
	GetPacketSource() *gopacket.PacketSource

	// GetStats returns capture statistics
	GetStats() CaptureStats
}

// EventLoop manages the main event processing loop
type EventLoop interface {
	io.Closer

	// Prepare initializes the event loop
	Prepare() error

	// Start begins the event loop
	Start(ctx context.Context, wg *sync.WaitGroup) error

	// ProcessPacket processes a single packet through handlers
	ProcessPacket(packet gopacket.Packet) error
}

// HandlerManager manages packet handlers
type HandlerManager interface {
	io.Closer

	// Prepare initializes the handler manager
	Prepare() error

	// Start begins handler processing
	Start(ctx context.Context, wg *sync.WaitGroup) error

	// RegisterHandler registers a new packet handler
	RegisterHandler(handler PacketHandler) error

	// UnregisterHandler removes a packet handler
	UnregisterHandler(name string) error

	// GetHandlers returns all registered handlers
	GetHandlers() []PacketHandler
}

// PacketHandler defines the interface for packet processing
type PacketHandler interface {
	// Name returns the handler name
	Name() string

	// CanHandle determines if this handler can process the packet
	CanHandle(packet gopacket.Packet) bool

	// Handle processes the packet and returns SniffData
	Handle(packet gopacket.Packet) ([]*v1.SniffData, error)

	// GetStats returns handler statistics
	GetStats() HandlerStats
}

// DataPipeline manages async data processing pipelines
type DataPipeline interface {
	io.Closer

	// Prepare initializes the data pipeline
	Prepare() error

	// Start begins pipeline processing
	Start(ctx context.Context, wg *sync.WaitGroup) error

	// Submit submits data for processing
	Submit(sniffData []*v1.SniffData) error

	// GetStats returns pipeline statistics
	GetStats() PipelineStats
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

// Statistics structures
type CaptureStats struct {
	PacketsReceived uint64
	PacketsDropped  uint64
	BytesReceived   uint64
	ErrorCount      uint64
}

type HandlerStats struct {
	PacketsProcessed uint64
	PacketsDropped   uint64
	ProcessingTime   time.Duration
	ErrorCount       uint64
}

type PipelineStats struct {
	ItemsProcessed uint64
	ItemsDropped   uint64
	QueueDepth     int
	ErrorCount     uint64
}
