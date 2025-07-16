package packet

import (
	"context"
	"sync"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
)

type Pipeline struct {
	source      types.DataSource
	filterChain types.FrameFilterChain
	dispatcher  types.FrameHandler

	// 控制流
	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup
}

func (p *Pipeline) Prepare() error {
	p.source.Prepare()
	return nil
}

func (p *Pipeline) Start(ctx context.Context, wg *sync.WaitGroup) error {

	for {
		select {
		case <-ctx.Done():
			log.Logger.Info("Pipeline context done, stopping...")
			return nil
		default:
			frame, err := p.source.Fetch(ctx)
			if err != nil {
				log.Logger.Error("Error fetching frame:", err)
				continue
			}

			p.filterChain.Filter(frame)
		}
	}
}

func (p *Pipeline) Close() error {
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
	return &Pipeline{
		source:      b.source,
		filterChain: b.filterChain,
		dispatcher:  b.dispatcher,
	}, nil
}
