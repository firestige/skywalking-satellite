package handler

import (
	"fmt"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/google/gopacket/layers"
)

type dispatcher struct {
	manager *Manager
}

func (d *dispatcher) Handle(frame *types.RawFrameData) error {
	protocol := frame.Connection.Protocol
	log.Logger.WithField("protocol", fmt.Sprintf("%v", protocol)).Debug("Dispatching frame")
	var handler types.FrameHandler
	var exists bool
	if handler, exists = d.manager.GetHandler(protocol, frame.Connection.SrcPort); !exists {
		if handler, exists = d.manager.GetHandler(protocol, frame.Connection.DstPort); !exists {
			log.Logger.WithField("protocol", protocol).Warn("No handler found for protocol")
			return fmt.Errorf("no handler found for protocol: %s", protocol)
		}
	}
	handler.Handle(frame)
	return nil
}

type DispatcherBuilder struct {
	Manager *Manager
	err     error
}

func NewDispatcherBuilder() *DispatcherBuilder {
	return &DispatcherBuilder{
		Manager: NewManager(false),
	}
}

func (b *DispatcherBuilder) WithHandler(protocol layers.IPProtocol, ports string, name string, handler func(frame *types.RawFrameData) error) *DispatcherBuilder {
	err := b.Manager.AddMapping(protocol, ports, name, handler)
	if err != nil {
		log.Logger.Error("Failed to add handler mapping: ", err)
		b.err = err
	}
	return b
}

func (b *DispatcherBuilder) Build() (types.FrameHandler, error) {
	if b.err != nil {
		return nil, fmt.Errorf("failed to build dispatcher: %w", b.err)
	}
	return &dispatcher{
		manager: b.Manager,
	}, nil
}
