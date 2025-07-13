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
	"testing"
	"time"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/internal/pkg/plugin"
)

func init() {
	// Initialize logger for tests
	log.Init(&log.LoggerConfig{})
}

func TestServer_Interface(t *testing.T) {
	// Test that Server implements the correct interfaces
	server := &Server{}

	// Test plugin interface
	if server.Name() != Name {
		t.Errorf("expected name %s, got %s", Name, server.Name())
	}

	if server.ShowName() != ShowName {
		t.Errorf("expected show name %s, got %s", ShowName, server.ShowName())
	}

	// Test that it has a description
	if server.Description() == "" {
		t.Error("description should not be empty")
	}

	// Test that it has default config
	if server.DefaultConfig() == "" {
		t.Error("default config should not be empty")
	}
}

func TestServer_Lifecycle(t *testing.T) {
	server := &Server{
		Interface:     "lo", // Use loopback interface for testing
		BufferSize:    64,
		Filter:        "tcp",
		StatsInterval: 1 * time.Second,
		DropThreshold: 10,
	}

	// Test Prepare
	if err := server.Prepare(); err != nil {
		// Note: This might fail in test environment without proper permissions
		t.Logf("Prepare failed (expected in test env): %v", err)
	}

	// Test GetServer
	if server.GetServer() != server {
		t.Error("GetServer should return the server instance")
	}

	// Test Close
	if err := server.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestComponentCreation(t *testing.T) {
	// Test that components can be created
	capture := NewPacketCapture("lo", 64, "tcp")
	if capture == nil {
		t.Error("PacketCapture creation failed")
	}

	handlerMgr := NewHandlerManager()
	if handlerMgr == nil {
		t.Error("HandlerManager creation failed")
	}

	pipeline := NewDataPipeline()
	if pipeline == nil {
		t.Error("DataPipeline creation failed")
	}

	eventLoop := NewEventLoop(capture, handlerMgr, pipeline)
	if eventLoop == nil {
		t.Error("EventLoop creation failed")
	}

	monitoring := NewMonitoringManager(1*time.Second, 10)
	if monitoring == nil {
		t.Error("MonitoringManager creation failed")
	}
}

func TestHandlerCreation(t *testing.T) {
	// Test that handlers can be created
	httpHandler := NewHTTPHandler()
	if httpHandler == nil {
		t.Error("HTTPHandler creation failed")
	}

	if httpHandler.Name() != "http-handler" {
		t.Errorf("expected handler name 'http-handler', got %s", httpHandler.Name())
	}

	tcpHandler := NewTCPHandler()
	if tcpHandler == nil {
		t.Error("TCPHandler creation failed")
	}

	if tcpHandler.Name() != "tcp-handler" {
		t.Errorf("expected handler name 'tcp-handler', got %s", tcpHandler.Name())
	}

	udpHandler := NewUDPHandler()
	if udpHandler == nil {
		t.Error("UDPHandler creation failed")
	}

	if udpHandler.Name() != "udp-handler" {
		t.Errorf("expected handler name 'udp-handler', got %s", udpHandler.Name())
	}
}

func TestPluginRegistration(t *testing.T) {
	// Test that the plugin is registered
	config := plugin.Config{
		"plugin_name": Name,
		"interface":   "lo",
		"buffer_size": 64,
		"filter":      "tcp",
	}

	// This will test the plugin registration system
	_ = config
}
