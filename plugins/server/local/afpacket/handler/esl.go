package handler

import (
	"github.com/google/gopacket"

	"github.com/apache/skywalking-satellite/plugins/server/local/afpacket/types"
)

const (
	Protocol = "ESL"
	Name     = "ESL"
	ShowName = "ELS Handler"
)

type eslHandler struct {
	stats types.HandlerStats
}

func NewESLHandler() types.PacketHandler {
	return &eslHandler{}
}

func (e *eslHandler) CanHandle(packet gopacket.Packet) bool {
	return false
}

func (e *eslHandler) Name() string {
	return Name
}

func (e *eslHandler) Type() string {
	return Protocol
}

func (e *eslHandler) Stats() types.HandlerStats {
	return e.stats
}

func (e *eslHandler) Handle(packet gopacket.Packet) ([]*types.RawFrameData, error) {
	return nil, nil
}
