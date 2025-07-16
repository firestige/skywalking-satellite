package packet

import (
	"github.com/apache/skywalking-satellite/internal/pkg/log"
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

func (f *frameFilterChain) Filter(frame types.RawFrameData) {
	log.Logger.Infof("Processing frame in chain: %s", string(frame.Data))
	if f.currentFilter != nil && f.chain != nil {
		f.currentFilter.Filter(frame, f.chain)
	} else {
		f.handler.Handle(frame)
	}
}
