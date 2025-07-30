package handler

import (
	"errors"
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
	src := fmt.Sprintf("%s:%d", frame.Connection.SrcHost, frame.Connection.SrcPort)
	dst := fmt.Sprintf("%s:%d", frame.Connection.DstHost, frame.Connection.DstPort)
	log.Logger.Tracef("Dispatching frame: {protocol: %s, src: %s, dst: %s}", protocol, src, dst)
	var handler types.FrameHandler
	var exists bool
	if handler, exists = d.manager.GetHandler(protocol, frame.Connection.SrcPort); !exists {
		if handler, exists = d.manager.GetHandler(protocol, frame.Connection.DstPort); !exists {
			log.Logger.Tracef("No handler found for frame: {protocol: %s, src: %s, dst: %s}, fallback to default", protocol, src, dst)
			if handler, exists = d.manager.GetDefaultHandlerByProtocol(protocol); !exists {
				msg := fmt.Sprintf("No handler found for frame: {protocol: %s, src: %s, dst: %s}", protocol, src, dst)
				log.Logger.Error(msg)
				return errors.New(msg)
			}
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
