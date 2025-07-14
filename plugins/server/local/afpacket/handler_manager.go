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
	"github.com/apache/skywalking-satellite/plugins/server/local/afpacket/handler"
)

// handlerManager implements HandlerManager interface
type handlerManager struct {
	handlers map[string]handler.PacketHandler
	mu       sync.RWMutex
}

// NewHandlerManager creates a new handler manager
func NewHandlerManager() HandlerManager {
	return &handlerManager{
		handlers: make(map[string]handler.PacketHandler),
	}
}

func (m *handlerManager) Prepare() error {
	log.Logger.Info("preparing handler manager...")

	// Initialize default handlers
	if err := m.initializeDefaultHandlers(); err != nil {
		return fmt.Errorf("failed to initialize default handlers: %w", err)
	}

	log.Logger.Info("handler manager prepared successfully")
	return nil
}

func (m *handlerManager) Start(ctx context.Context, wg *sync.WaitGroup) error {
	log.Logger.Info("starting handler manager...")

	// Start all registered handlers
	for name, handler := range m.handlers {
		log.Logger.Infof("starting handler: %s", name)
		// TODO: Start handler if needed
		_ = handler
	}

	return nil
}

func (m *handlerManager) Close() error {
	log.Logger.Info("closing handler manager...")

	m.mu.Lock()
	defer m.mu.Unlock()

	// Close all handlers
	for name, handler := range m.handlers {
		log.Logger.Infof("closing handler: %s", name)
		// TODO: Close handler if needed
		_ = handler
	}

	return nil
}

func (m *handlerManager) RegisterHandler(handler handler.PacketHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := handler.Name()
	if _, exists := m.handlers[name]; exists {
		return fmt.Errorf("handler %s already exists", name)
	}

	m.handlers[name] = handler
	log.Logger.Infof("registered handler: %s", name)
	return nil
}

func (m *handlerManager) UnregisterHandler(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.handlers[name]; !exists {
		return fmt.Errorf("handler %s not found", name)
	}

	delete(m.handlers, name)
	log.Logger.Infof("unregistered handler: %s", name)
	return nil
}

func (m *handlerManager) GetHandlers() []handler.PacketHandler {
	m.mu.RLock()
	defer m.mu.RUnlock()

	handlers := make([]handler.PacketHandler, 0, len(m.handlers))
	for _, handler := range m.handlers {
		handlers = append(handlers, handler)
	}

	return handlers
}

func (m *handlerManager) initializeDefaultHandlers() error {
	// Define default handlers
	handlers := []handler.PacketHandler{
		handler.NewHTTPHandler(),
		handler.NewESLHandler(),
	}

	// Register all handlers
	for _, h := range handlers {
		if err := m.RegisterHandler(h); err != nil {
			return fmt.Errorf("failed to register %s handler: %w", h.Name(), err)
		}
	}

	return nil
}
