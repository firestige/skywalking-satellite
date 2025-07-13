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
	"sync"
	"time"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
)

// monitoringManager implements MonitoringManager interface
type monitoringManager struct {
	statsInterval time.Duration
	dropThreshold int

	// Statistics tracking
	dropCounts    map[string]int
	processCounts map[string]int
	mu            sync.RWMutex

	// Lifecycle
	ticker *time.Ticker
}

// NewMonitoringManager creates a new monitoring manager
func NewMonitoringManager(statsInterval time.Duration, dropThreshold int) MonitoringManager {
	return &monitoringManager{
		statsInterval: statsInterval,
		dropThreshold: dropThreshold,
		dropCounts:    make(map[string]int),
		processCounts: make(map[string]int),
	}
}

func (m *monitoringManager) Prepare() error {
	log.Logger.Info("preparing monitoring manager...")

	// Initialize ticker for periodic stats reporting
	m.ticker = time.NewTicker(m.statsInterval)

	log.Logger.Info("monitoring manager prepared successfully")
	return nil
}

func (m *monitoringManager) Start(ctx context.Context, wg *sync.WaitGroup) error {
	log.Logger.Info("starting monitoring manager...")

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.run(ctx)
	}()

	return nil
}

func (m *monitoringManager) Close() error {
	log.Logger.Info("closing monitoring manager...")

	if m.ticker != nil {
		m.ticker.Stop()
	}

	return nil
}

func (m *monitoringManager) ReportStats() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Report current statistics
	log.Logger.Info("=== AFPacket Server Statistics ===")

	// Report drop counts
	for handlerName, count := range m.dropCounts {
		log.Logger.Infof("Handler %s: %d packets dropped", handlerName, count)

		// Check if exceeds threshold
		if count > m.dropThreshold {
			log.Logger.Warnf("Handler %s exceeds drop threshold (%d > %d)",
				handlerName, count, m.dropThreshold)
		}
	}

	// Report process counts
	for handlerName, count := range m.processCounts {
		log.Logger.Infof("Handler %s: %d packets processed", handlerName, count)
	}

	return nil
}

func (m *monitoringManager) RecordDrop(handlerName string, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.dropCounts[handlerName]++

	log.Logger.Debugf("Packet dropped by handler %s: %s", handlerName, reason)
}

func (m *monitoringManager) RecordProcess(handlerName string, processingTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.processCounts[handlerName]++

	log.Logger.Debugf("Packet processed by handler %s in %v", handlerName, processingTime)
}

func (m *monitoringManager) run(ctx context.Context) {
	log.Logger.Info("monitoring manager started")
	defer log.Logger.Info("monitoring manager stopped")

	for {
		select {
		case <-ctx.Done():
			// Final stats report before shutdown
			m.ReportStats()
			return
		case <-m.ticker.C:
			// Periodic stats reporting
			if err := m.ReportStats(); err != nil {
				log.Logger.Errorf("failed to report stats: %v", err)
			}

			// Reset counters after reporting
			m.resetCounters()
		}
	}
}

func (m *monitoringManager) resetCounters() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Reset drop counts
	for k := range m.dropCounts {
		m.dropCounts[k] = 0
	}

	// Reset process counts
	for k := range m.processCounts {
		m.processCounts[k] = 0
	}
}
