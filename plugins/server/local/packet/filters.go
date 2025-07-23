package packet

import (
	"context"

	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
)

type frameFilterChain struct {
	filters       []types.FrameFilter
	currentFilter types.FrameFilter
	handler       types.FrameHandler
	chain         types.FrameFilterChain
}

// Ensure frameFilterChain implements types.FrameFilterChain interface
var _ types.FrameFilterChain = (*frameFilterChain)(nil)

func NewFrameFilterChain(handler types.FrameHandler, filters []types.FrameFilter) types.FrameFilterChain {
	var chain types.FrameFilterChain
	chain = &frameFilterChain{
		filters:       filters,
		handler:       handler,
		currentFilter: nil,
		chain:         nil,
	}
	for i := len(filters); i > 0; i-- {
		chain = &frameFilterChain{
			filters:       filters,
			handler:       handler,
			currentFilter: filters[i-1],
			chain:         chain,
		}
	}
	return chain
}

func (f *frameFilterChain) Filters() []types.FrameFilter {
	return f.filters
}

func (f *frameFilterChain) Handler() types.FrameHandler {
	return f.handler
}

func (f *frameFilterChain) Filter(frame *types.RawFrameData) {
	if f.currentFilter != nil && f.chain != nil {
		f.currentFilter.Filter(frame, f.chain)
	} else {
		f.handler.Handle(frame)
	}
}

func (f *frameFilterChain) Prepare(ctx context.Context) error {
	for _, filter := range f.filters {
		if err := filter.Prepare(ctx); err != nil {
			return err
		}
	}
	if f.handler != nil {
		if err := f.handler.Prepare(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (f *frameFilterChain) Close() error {
	for _, filter := range f.filters {
		if err := filter.Close(); err != nil {
			return err
		}
	}
	if f.handler != nil {
		if err := f.handler.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (f *frameFilterChain) Start() error {
	return nil
}
