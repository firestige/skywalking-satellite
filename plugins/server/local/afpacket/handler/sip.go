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
	"github.com/google/gopacket/layers"

	"github.com/apache/skywalking-satellite/plugins/server/local/afpacket/types"
)

type sipHandler struct {
	stats types.HandlerStats
}

func NewSIPHandler() types.PacketHandler {
	return &sipHandler{}
}

func (s *sipHandler) CanHandle(packet gopacket.Packet) bool {
	if sip := packet.Layer(layers.LayerTypeSIP); sip != nil {
		return true // 简单实现，实际需要更复杂的逻辑
	}
	// 如果没有 SIP 层，返回 false
	return false
}

func (s *sipHandler) Name() string {
	return "sip"
}

func (s *sipHandler) Type() string {
	return "sip"
}

func (s *sipHandler) Stats() types.HandlerStats {
	return s.stats
}

func (s *sipHandler) Handle(packet gopacket.Packet) ([]*types.RawFrameData, error) {
	data := &types.RawFrameData{
		Protocol:  "SIP",
		Content:   packet.Layer(layers.LayerTypeSIP).LayerContents(),
		Timestamp: packet.Metadata().Timestamp.UnixNano() / 1e6,
	}
	return []*types.RawFrameData{data}, nil
}
