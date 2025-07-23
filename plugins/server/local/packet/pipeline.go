package packet

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/filter"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/sirupsen/logrus"
)

type Pipeline struct {
	source      types.DataSource
	filterChain types.FrameFilterChain
	dispatcher  types.FrameHandler

	// 控制流
	ctx context.Context
	wg  *sync.WaitGroup
}

func (p *Pipeline) Prepare() error {
	log.Logger.WithField("pipeline", Name).Debug("Preparing pipeline...")
	// 准备数据源
	if err := p.source.Prepare(); err != nil {
		return err
	}

	return nil
}

func (p *Pipeline) Start() error {

	log.Logger.Info("Starting pipeline...")
	p.wg.Add(1)
	go p.run()

	return nil
}

func (p *Pipeline) run() {
	defer func() {
		p.wg.Done()
		if r := recover(); r != nil {
			log.Logger.WithFields(logrus.Fields{
				"error": r,
				"stack": string(debug.Stack()),
			}).Error("Pipeline panic recovered")
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
			frame, err := p.source.Fetch()
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
	ctx         context.Context
}

func NewPipelineBuilder(dispatcher types.FrameHandler, ctx context.Context) *PipelineBuilder {
	return &PipelineBuilder{dispatcher: dispatcher, ctx: ctx}
}

func (b *PipelineBuilder) WithSource(source types.DataSource) *PipelineBuilder {
	b.source = source
	return b
}

func (b *PipelineBuilder) WithFilters(filters []types.FrameFilter) *PipelineBuilder {
	b.filterChain = filter.NewFrameFilterChain(b.dispatcher, filters)
	return b
}

func (b *PipelineBuilder) Build() (*Pipeline, error) {
	if b.source == nil {
		return nil, fmt.Errorf("data source is required")
	}

	if b.filterChain == nil {
		// 如果没有过滤器，创建一个空的过滤链
		b.filterChain = filter.NewFrameFilterChain(b.dispatcher, []types.FrameFilter{})
	}

	if b.dispatcher == nil {
		return nil, fmt.Errorf("frame handler is required")
	}

	return &Pipeline{
		source:      b.source,
		filterChain: b.filterChain,
		dispatcher:  b.dispatcher,
		ctx:         b.ctx,
		wg:          &sync.WaitGroup{},
	}, nil
}
