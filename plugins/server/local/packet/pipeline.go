package packet

import (
	"context"
	"fmt"
	"sync"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
)

type Pipeline struct {
	source      types.DataSource
	filterChain types.FrameFilterChain
	dispatcher  types.FrameHandler

	// 控制流
	ctx context.Context
	wg  *sync.WaitGroup
}

func (p *Pipeline) Prepare(ctx context.Context) error {
	// 设置上下文和等待组
	p.ctx = ctx
	p.wg = &sync.WaitGroup{}

	// 准备数据源
	if err := p.source.Prepare(ctx); err != nil {
		return err
	}

	return nil
}

func (p *Pipeline) Start() error {
	// 异步启动流水线处理
	p.wg.Add(1)
	go p.run()

	return nil
}

func (p *Pipeline) run() {
	defer p.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			log.Logger.Error("Pipeline panic recovered:", r)
		}
	}()

	log.Logger.Info("Pipeline started")
	if err := p.source.Start(); err != nil {
		log.Logger.Error("Error starting source:", err)
		return
	}

	for {
		select {
		case <-p.ctx.Done():
			log.Logger.Info("Pipeline context done, stopping...")
			return
		default:
			frame, err := p.source.Fetch(p.ctx)
			if err != nil {
				log.Logger.Error("Error fetching frame:", err)
				continue
			}

			if frame == types.EmptyRawFrameData {
				continue
			}

			// 通过过滤链处理frame
			p.filterChain.Filter(frame)
		}
	}
}

func (p *Pipeline) Close() error {
	log.Logger.Info("Closing pipeline...")

	// 等待所有 goroutine 完成
	if p.wg != nil {
		p.wg.Wait()
	}

	// 关闭数据源
	if err := p.source.Close(); err != nil {
		log.Logger.Error("Error closing source:", err)
	}

	log.Logger.Info("Pipeline closed")
	return nil
}

type PipelineBuilder struct {
	source      types.DataSource
	filterChain types.FrameFilterChain
	dispatcher  types.FrameHandler
}

func NewPipelineBuilder(dispatcher types.FrameHandler) *PipelineBuilder {
	return &PipelineBuilder{dispatcher: dispatcher}
}

func (b *PipelineBuilder) WithSource(source types.DataSource) *PipelineBuilder {
	b.source = source
	return b
}

func (b *PipelineBuilder) WithFilters(filters []types.FrameFilter) *PipelineBuilder {
	b.filterChain = NewFrameFilterChain(b.dispatcher, filters)
	return b
}

func (b *PipelineBuilder) Build() (*Pipeline, error) {
	if b.source == nil {
		return nil, fmt.Errorf("data source is required")
	}

	if b.filterChain == nil {
		// 如果没有过滤器，创建一个空的过滤链
		b.filterChain = NewFrameFilterChain(b.dispatcher, []types.FrameFilter{})
	}

	if b.dispatcher == nil {
		return nil, fmt.Errorf("frame handler is required")
	}

	return &Pipeline{
		source:      b.source,
		filterChain: b.filterChain,
		dispatcher:  b.dispatcher,
	}, nil
}
