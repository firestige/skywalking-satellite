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

package handler

import (
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPHandler_Name(t *testing.T) {
	handler := NewHTTPHandler()
	assert.Equal(t, "http", handler.Name())
}

func TestHTTPHandler_GetType(t *testing.T) {
	handler := NewHTTPHandler()
	assert.Equal(t, "http", handler.Type())
}

func TestHTTPHandler_CanHandle(t *testing.T) {
	handler := NewHTTPHandler()

	// Create a mock packet with application layer
	packet := gopacket.NewPacket(
		[]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"),
		layers.LayerTypeEthernet,
		gopacket.Default,
	)

	// For now, this should return true if application layer exists
	// In real implementation, this would need proper HTTP detection
	result := handler.CanHandle(packet)
	assert.True(t, result)
}

func TestHTTPHandler_Handle(t *testing.T) {
	handler := NewHTTPHandler()

	// Create a mock packet
	packet := gopacket.NewPacket(
		[]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"),
		layers.LayerTypeEthernet,
		gopacket.Default,
	)

	sniffData, err := handler.Handle(packet)
	require.NoError(t, err)
	require.NotNil(t, sniffData)
	assert.Len(t, sniffData, 1)
	assert.Equal(t, "http-data", sniffData[0].Protocol)
}

func TestHTTPHandler_GetStats(t *testing.T) {
	handler := NewHTTPHandler()
	stats := handler.Stats()
	assert.Equal(t, uint64(0), stats.PacketsHandled)
	assert.Equal(t, uint64(0), stats.DataGenerated)
	assert.Equal(t, uint64(0), stats.ErrorCount)
}
