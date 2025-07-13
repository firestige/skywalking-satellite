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
)

// handlerManager implements HandlerManager interface
type handlerManager struct {
	handlers map[string]PacketHandler
	mu       sync.RWMutex
}

// NewHandlerManager creates a new handler manager
func NewHandlerManager() HandlerManager {
	return &handlerManager{
		handlers: make(map[string]PacketHandler),
	}
}

func (h *handlerManager) Prepare() error {
	log.Logger.Info("preparing handler manager...")

	// Initialize default handlers
	if err := h.initializeDefaultHandlers(); err != nil {
		return fmt.Errorf("failed to initialize default handlers: %w", err)
	}

	log.Logger.Info("handler manager prepared successfully")
	return nil
}

func (h *handlerManager) Start(ctx context.Context, wg *sync.WaitGroup) error {
	log.Logger.Info("starting handler manager...")

	// Start all registered handlers
	for name, handler := range h.handlers {
		log.Logger.Infof("starting handler: %s", name)
		// TODO: Start handler if needed
		_ = handler
	}

	return nil
}

func (h *handlerManager) Close() error {
	log.Logger.Info("closing handler manager...")

	h.mu.Lock()
	defer h.mu.Unlock()

	// Close all handlers
	for name, handler := range h.handlers {
		log.Logger.Infof("closing handler: %s", name)
		// TODO: Close handler if needed
		_ = handler
	}

	return nil
}

func (h *handlerManager) RegisterHandler(handler PacketHandler) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	name := handler.Name()
	if _, exists := h.handlers[name]; exists {
		return fmt.Errorf("handler %s already exists", name)
	}

	h.handlers[name] = handler
	log.Logger.Infof("registered handler: %s", name)
	return nil
}

func (h *handlerManager) UnregisterHandler(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.handlers[name]; !exists {
		return fmt.Errorf("handler %s not found", name)
	}

	delete(h.handlers, name)
	log.Logger.Infof("unregistered handler: %s", name)
	return nil
}

func (h *handlerManager) GetHandlers() []PacketHandler {
	h.mu.RLock()
	defer h.mu.RUnlock()

	handlers := make([]PacketHandler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler)
	}

	return handlers
}

func (h *handlerManager) initializeDefaultHandlers() error {
	// Register default handlers

	// HTTP Handler
	httpHandler := NewHTTPHandler()
	if err := h.RegisterHandler(httpHandler); err != nil {
		return fmt.Errorf("failed to register HTTP handler: %w", err)
	}

	// TCP Handler
	tcpHandler := NewTCPHandler()
	if err := h.RegisterHandler(tcpHandler); err != nil {
		return fmt.Errorf("failed to register TCP handler: %w", err)
	}

	// UDP Handler
	udpHandler := NewUDPHandler()
	if err := h.RegisterHandler(udpHandler); err != nil {
		return fmt.Errorf("failed to register UDP handler: %w", err)
	}

	return nil
}
