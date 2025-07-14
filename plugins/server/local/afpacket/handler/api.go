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
	"github.com/google/gopacket"
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"
)

// PacketHandler defines the interface for packet handlers
type PacketHandler interface {
	Handle(packet gopacket.Packet) ([]*v1.SniffData, error)
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
