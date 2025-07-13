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
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"
)

// dataPipeline implements DataPipeline interface
type dataPipeline struct {
	// Multiple independent pipelines for different data types
	pipelines map[string]*pipeline
	mu        sync.RWMutex

	// Configuration
	defaultBufferSize int
	maxWorkers        int

	// Statistics
	stats PipelineStats
}

// pipeline represents a single data processing pipeline
type pipeline struct {
	name       string
	dataChan   chan []*v1.SniffData
	workerPool chan struct{}
	stats      PipelineStats
	mu         sync.RWMutex
}

// NewDataPipeline creates a new data pipeline
func NewDataPipeline() DataPipeline {
	return &dataPipeline{
		pipelines:         make(map[string]*pipeline),
		defaultBufferSize: 1000,
		maxWorkers:        10,
	}
}

func (d *dataPipeline) Prepare() error {
	log.Logger.Info("preparing data pipeline...")

	// Initialize default pipelines
	if err := d.initializePipelines(); err != nil {
		return fmt.Errorf("failed to initialize pipelines: %w", err)
	}

	log.Logger.Info("data pipeline prepared successfully")
	return nil
}

func (d *dataPipeline) Start(ctx context.Context, wg *sync.WaitGroup) error {
	log.Logger.Info("starting data pipeline...")

	d.mu.RLock()
	defer d.mu.RUnlock()

	// Start all pipelines
	for name, pipelineInstance := range d.pipelines {
		log.Logger.Infof("starting pipeline: %s", name)

		wg.Add(1)
		go func(p *pipeline) {
			defer wg.Done()
			d.runPipeline(ctx, p)
		}(pipelineInstance)
	}

	return nil
}

func (d *dataPipeline) Close() error {
	log.Logger.Info("closing data pipeline...")

	d.mu.Lock()
	defer d.mu.Unlock()

	// Close all pipeline channels
	for name, pipeline := range d.pipelines {
		log.Logger.Infof("closing pipeline: %s", name)
		close(pipeline.dataChan)
	}

	return nil
}

func (d *dataPipeline) Submit(sniffData []*v1.SniffData) error {
	if len(sniffData) == 0 {
		return nil
	}

	// Determine which pipeline to use based on data type
	// For now, use a default pipeline
	pipelineName := "default"

	d.mu.RLock()
	pipeline, exists := d.pipelines[pipelineName]
	d.mu.RUnlock()

	if !exists {
		return fmt.Errorf("pipeline %s not found", pipelineName)
	}

	// Non-blocking send with drop policy
	select {
	case pipeline.dataChan <- sniffData:
		// Successfully submitted
		return nil
	default:
		// Channel is full, drop the data
		pipeline.mu.Lock()
		pipeline.stats.ItemsDropped += uint64(len(sniffData))
		pipeline.mu.Unlock()

		log.Logger.Warnf("pipeline %s is full, dropping %d items", pipelineName, len(sniffData))
		return nil
	}
}

func (d *dataPipeline) GetStats() PipelineStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Aggregate stats from all pipelines
	totalStats := PipelineStats{}
	for _, pipeline := range d.pipelines {
		pipeline.mu.RLock()
		totalStats.ItemsProcessed += pipeline.stats.ItemsProcessed
		totalStats.ItemsDropped += pipeline.stats.ItemsDropped
		totalStats.QueueDepth += pipeline.stats.QueueDepth
		totalStats.ErrorCount += pipeline.stats.ErrorCount
		pipeline.mu.RUnlock()
	}

	return totalStats
}

func (d *dataPipeline) initializePipelines() error {
	// Create default pipeline
	defaultPipeline := &pipeline{
		name:       "default",
		dataChan:   make(chan []*v1.SniffData, d.defaultBufferSize),
		workerPool: make(chan struct{}, d.maxWorkers),
	}

	d.pipelines["default"] = defaultPipeline

	// Initialize HTTP pipeline
	httpPipeline := &pipeline{
		name:       "http",
		dataChan:   make(chan []*v1.SniffData, d.defaultBufferSize),
		workerPool: make(chan struct{}, d.maxWorkers),
	}

	d.pipelines["http"] = httpPipeline

	// Initialize TCP pipeline
	tcpPipeline := &pipeline{
		name:       "tcp",
		dataChan:   make(chan []*v1.SniffData, d.defaultBufferSize),
		workerPool: make(chan struct{}, d.maxWorkers),
	}

	d.pipelines["tcp"] = tcpPipeline

	return nil
}

func (d *dataPipeline) runPipeline(ctx context.Context, p *pipeline) {
	log.Logger.Infof("pipeline %s started", p.name)
	defer log.Logger.Infof("pipeline %s stopped", p.name)

	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-p.dataChan:
			if !ok {
				return
			}

			// Process data
			if err := d.processData(p, data); err != nil {
				log.Logger.Errorf("failed to process data in pipeline %s: %v", p.name, err)

				p.mu.Lock()
				p.stats.ErrorCount++
				p.mu.Unlock()
			}
		}
	}
}

func (d *dataPipeline) processData(p *pipeline, data []*v1.SniffData) error {
	// Acquire worker from pool
	p.workerPool <- struct{}{}
	defer func() { <-p.workerPool }()

	// Process each SniffData
	for _, sniffData := range data {
		// TODO: Send to registered receiver
		// For now, just log
		log.Logger.Debugf("processing sniff data: %s", sniffData.Name)
	}

	// Update statistics
	p.mu.Lock()
	p.stats.ItemsProcessed += uint64(len(data))
	p.stats.QueueDepth = len(p.dataChan)
	p.mu.Unlock()

	return nil
}
