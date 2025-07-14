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
	"github.com/apache/skywalking-satellite/plugins/server/local/afpacket/types"
	"github.com/google/gopacket"
)

// eventLoop implements EventLoop interface
type eventLoop struct {
	capture    types.PacketCapture
	handlerMgr types.HandlerManager
	pipeline   types.DataPipeline

	running bool
	mu      sync.RWMutex
}

// NewEventLoop creates a new event loop
func NewEventLoop(capture types.PacketCapture, handlerMgr types.HandlerManager, pipeline types.DataPipeline) types.EventLoop {
	return &eventLoop{
		capture:    capture,
		handlerMgr: handlerMgr,
		pipeline:   pipeline,
	}
}

func (e *eventLoop) Prepare() error {
	log.Logger.Info("preparing event loop...")

	// 验证必要的组件依赖
	if e.capture == nil {
		return fmt.Errorf("packet capture is nil")
	}
	if e.handlerMgr == nil {
		return fmt.Errorf("handler manager is nil")
	}
	if e.pipeline == nil {
		return fmt.Errorf("data pipeline is nil")
	}

	// 验证组件是否实现了必要的接口方法
	if _, ok := e.capture.(interface{ GetPacketChannel() <-chan gopacket.Packet }); !ok {
		return fmt.Errorf("packet capture does not implement GetPacketChannel method")
	}

	log.Logger.Info("event loop prepared successfully")
	return nil
}

func (e *eventLoop) Start(ctx context.Context, wg *sync.WaitGroup) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("event loop is already running")
	}

	log.Logger.Info("starting event loop...")

	wg.Add(1)
	go func() {
		defer wg.Done()
		e.run(ctx)
	}()

	e.running = true
	return nil
}

func (e *eventLoop) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	log.Logger.Info("closing event loop...")
	e.running = false

	return nil
}

func (e *eventLoop) ProcessPacket(packet gopacket.Packet) error {
	// Get all handlers from handler manager
	handlers := e.handlerMgr.GetHandlers()

	// Process packet through each capable handler
	for _, handler := range handlers {
		if handler.CanHandle(packet) {
			rawData, err := handler.Handle(packet)
			if err != nil {
				log.Logger.Errorf("handler %s failed to process packet: %v", handler.Name(), err)
				continue
			}

			// Submit to pipeline for async processing
			if err := e.pipeline.Submit(rawData); err != nil {
				log.Logger.Errorf("failed to submit data to pipeline: %v", err)
			}
		}
	}

	return nil
}

func (e *eventLoop) run(ctx context.Context) {
	log.Logger.Info("event loop started")
	defer log.Logger.Info("event loop stopped")

	// 使用接口方法获取通道
	packetChan := e.capture.GetPacketChannel()

	for {
		select {
		case <-ctx.Done():
			return
		case packet, ok := <-packetChan:
			if !ok {
				log.Logger.Info("packet channel closed")
				return
			}

			if err := e.ProcessPacket(packet); err != nil {
				log.Logger.Errorf("failed to process packet: %v", err)
			}
		}
	}
}
