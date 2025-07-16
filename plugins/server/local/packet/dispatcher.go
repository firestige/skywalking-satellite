package packet

import (
	"fmt"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
)

type frameHandlerAdapter struct {
	name    string // Handler name for logging
	handler func(frame types.RawFrameData) error
}

func newFrameHandlerAdapter(name string, handler func(frame types.RawFrameData) error) *frameHandlerAdapter {
	if handler == nil {
		log.Logger.Error("Handler function cannot be nil")
		return nil
	}
	return &frameHandlerAdapter{name: name, handler: handler}
}

func (a *frameHandlerAdapter) Handle(frame types.RawFrameData) error {
	return a.handler(frame)
}

func (a *frameHandlerAdapter) Name() string {
	return a.name
}

type dispatcher struct {
	handlerMapping map[string]types.FrameHandler
}

func (d *dispatcher) Handle(frame types.RawFrameData) error {
	protocol := frame.Connection.Protocol
	if handler, exists := d.handlerMapping[protocol]; exists {
		handler.Handle(frame)
		return nil
	} else {
		return fmt.Errorf("no handler found for protocol: %s", protocol)
	}
}

type DispatcherBuilder struct {
	handlerMapping map[string]*frameHandlerAdapter
}

func NewDispatcherBuilder() *DispatcherBuilder {
	return &DispatcherBuilder{
		handlerMapping: make(map[string]*frameHandlerAdapter),
	}
}

func (b *DispatcherBuilder) WithHandler(protocol string, name string, handler func(frame types.RawFrameData) error) *DispatcherBuilder {
	if protocol == "" || handler == nil {
		log.Logger.Error("Protocol and handler must be provided")
		return b
	}
	if old, exists := b.handlerMapping[protocol]; exists {
		log.Logger.Warnf("Handler for protocol %s already exists, replacing: %T -> %T", protocol, old.Name(), name)
	}
	b.handlerMapping[protocol] = newFrameHandlerAdapter(name, handler)
	return b
}

func (b *DispatcherBuilder) Build() (types.FrameHandler, error) {
	if len(b.handlerMapping) == 0 {
		return nil, fmt.Errorf("no handlers defined for dispatcher")
	}
	// Convert map[string]*frameHandlerAdapter to map[string]types.FrameHandler
	handlerMapping := make(map[string]types.FrameHandler, len(b.handlerMapping))
	for k, v := range b.handlerMapping {
		handlerMapping[k] = v
	}
	return &dispatcher{
		handlerMapping: handlerMapping,
	}, nil
}
