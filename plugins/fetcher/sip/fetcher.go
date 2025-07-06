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

package sip

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"time"

	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"

	"github.com/google/gopacket"
	"github.com/google/gopacket/afpacket"
	"github.com/google/gopacket/layers"

	"github.com/apache/skywalking-satellite/internal/pkg/config"
	forwarder "github.com/apache/skywalking-satellite/plugins/forwarder/api"
	forwarder_grpc "github.com/apache/skywalking-satellite/plugins/forwarder/grpc"
	forwarder_kafka "github.com/apache/skywalking-satellite/plugins/forwarder/kafka"
)

const (
	// Name is the name of the SIP fetcher.
	Name     = "sip-fetcher"
	ShowName = "SIP Fetcher"
)

type Fetcher struct {
	config.CommonFields
	// congfig
	NicName string `mapstructure:"nic_name"` // Network interface name to listen on, e.g., "eth0".
	// components
	Handle *afpacket.TPacket
	// OutputChannel is the channel where fetched SIP data will be sent.
	OutputChannel chan *v1.SniffData
}

func (f *Fetcher) Name() string {
	return Name
}

func (f *Fetcher) ShowName() string {
	return ShowName
}

func (f *Fetcher) Description() string {
	return "SIP Fetcher captures SIP protocol traffic and processes it for analysis."
}

func (f *Fetcher) DefaultConfig() string {
	return `
nic_name: eth0  # The network interface to capture traffic from
`
}

func (f *Fetcher) Prepare() {
	var err error
	f.OutputChannel = make(chan *v1.SniffData, 1000) // 初始化channel并设置合适的buffer大小
	f.Handle, err = afpacket.NewTPacket(
		afpacket.OptInterface(f.NicName),
		afpacket.OptFrameSize(65536),
		afpacket.TPacketVersion3,
	)
	if err != nil {
		log.Fatalf("Error creating af_packet handle: %v", err)
	}
}

func (f *Fetcher) Fetch(ctx context.Context) {
	packetSource := gopacket.NewPacketSource(f.Handle, layers.LinkTypeEthernet)
	for packet := range packetSource.Packets() {
		if sipLayer := packet.Layer(layers.LayerTypeSIP); sipLayer != nil {
			s, ok := sipLayer.(*layers.SIP)
			if !ok {
				continue
			}

			// 提取SIP消息的关键信息
			timestamp := packet.Metadata().Timestamp
			srcIP, dstIP := extractIPs(packet)

			// Create segment with SIP data

			sniffData := &v1.SniffData{
				Name: "sipraw-" + generateTraceSegmentID(), // Unique identifier for the SIP message
				Data: &v1.SniffData_Segment{
					Segment: segment,
				},
			}

			select {
			case f.OutputChannel <- sniffData:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (f *Fetcher) Channel() <-chan *v1.SniffData {
	// Return a channel that will receive fetched data.
	// This is a placeholder; actual implementation should return a channel that receives SIP messages.
	return f.OutputChannel
}

func (f *Fetcher) Shutdown(ctx context.Context) error {
	// Implement the logic to gracefully shutdown the SIP fetcher.
	// This might include closing connections or cleaning up resources.
	if f.Handle != nil {
		f.Handle.Close()
	}
	close(f.OutputChannel)
	return nil
}

func (f *Fetcher) SupportForwarders() []forwarder.Forwarder {
	// Return a list of forwarders that this fetcher supports.
	// This is a placeholder; actual implementation should return the forwarders that can handle SIP data.
	return []forwarder.Forwarder{
		new(forwarder_grpc.Forwarder),  // For sending to OAP
		new(forwarder_kafka.Forwarder), // For async storage
	}
}

// 辅助函数：从数据包提取源IP和目标IP
func extractIPs(packet gopacket.Packet) (string, string) {
	var srcIP, dstIP string

	// 尝试IPv4
	if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
		ip, _ := ipLayer.(*layers.IPv4)
		srcIP = ip.SrcIP.String()
		dstIP = ip.DstIP.String()
		return srcIP, dstIP
	}

	// 尝试IPv6
	if ipLayer := packet.Layer(layers.LayerTypeIPv6); ipLayer != nil {
		ip, _ := ipLayer.(*layers.IPv6)
		srcIP = ip.SrcIP.String()
		dstIP = ip.DstIP.String()
		return srcIP, dstIP
	}

	return "unknown", "unknown"
}

// 生成TraceID，使用CallID作为基础确保同一会话的消息具有相同的TraceID
func generateTraceID(callID string) string {
	hash := sha256.Sum256([]byte(callID))
	return hex.EncodeToString(hash[:16]) // 使用前16字节作为TraceID
}

// 生成SpanID
func generateSpanID() string {
	random := make([]byte, 8)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func generateTraceSegmentID() string {
	random := make([]byte, 16)
	rand.Read(random)
	return hex.EncodeToString(random)
}

func generateTimeBucket(t time.Time) int64 {
	return t.Unix() / 60 * 60 // 按分钟生成时间桶
}
