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

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// packetCapture implements PacketCapture interface
type packetCapture struct {
	interfaceName string
	bufferSize    int
	filter        string

	handle       *pcap.Handle
	packetSource *gopacket.PacketSource
	stats        CaptureStats
	mu           sync.RWMutex

	// 添加包分发通道
	packetChan chan gopacket.Packet
	closed     chan struct{} // 添加关闭信号
}

// NewPacketCapture creates a new packet capture instance
func NewPacketCapture(interfaceName string, bufferSize int, filter string) PacketCapture {
	return &packetCapture{
		interfaceName: interfaceName,
		bufferSize:    bufferSize,
		filter:        filter,
		packetChan:    make(chan gopacket.Packet, 100),
		closed:        make(chan struct{}),
	}
}

func (p *packetCapture) Prepare() error {
	log.Logger.Infof("preparing packet capture on interface: %s", p.interfaceName)

	// Open the interface for packet capture
	handle, err := pcap.OpenLive(p.interfaceName, int32(p.bufferSize), true, pcap.BlockForever)
	if err != nil {
		return fmt.Errorf("failed to open interface %s: %w", p.interfaceName, err)
	}

	// Set BPF filter if specified
	if p.filter != "" {
		if err := handle.SetBPFFilter(p.filter); err != nil {
			handle.Close()
			return fmt.Errorf("failed to set BPF filter %s: %w", p.filter, err)
		}
	}

	p.handle = handle
	p.packetSource = gopacket.NewPacketSource(handle, layers.LinkTypeEthernet)

	log.Logger.Info("packet capture prepared successfully")
	return nil
}

func (p *packetCapture) Start(ctx context.Context, wg *sync.WaitGroup) error {
	log.Logger.Info("starting packet capture...")

	wg.Add(1)
	go func() {
		defer wg.Done()
		p.captureLoop(ctx)
	}()

	return nil
}

func (p *packetCapture) Close() error {
	log.Logger.Info("closing packet capture...")

	// 发送关闭信号
	close(p.closed)

	if p.handle != nil {
		p.handle.Close()
	}

	return nil
}

func (p *packetCapture) GetStats() CaptureStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

func (p *packetCapture) captureLoop(ctx context.Context) {
	log.Logger.Info("packet capture loop started")
	defer log.Logger.Info("packet capture loop stopped")
	defer close(p.packetChan) // 确保在退出时关闭通道

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.closed:
			return
		default:
			packet, err := p.packetSource.NextPacket()
			if err != nil {
				log.Logger.Errorf("error reading packet: %v", err)
				p.mu.Lock()
				p.stats.ErrorCount++
				p.mu.Unlock()
				continue
			}

			// 更新统计信息
			p.mu.Lock()
			p.stats.PacketsReceived++
			p.stats.BytesReceived += uint64(len(packet.Data()))
			p.mu.Unlock()

			// 非阻塞发送到处理通道
			select {
			case p.packetChan <- packet:
			default:
				// 如果通道满了，丢弃包并记录统计
				p.mu.Lock()
				p.stats.PacketsDropped++
				p.mu.Unlock()
				log.Logger.Warn("packet channel full, dropping packet")
			case <-ctx.Done():
				return
			case <-p.closed:
				return
			}
		}
	}
}

func (p *packetCapture) GetPacketChannel() <-chan gopacket.Packet {
	return p.packetChan
}
