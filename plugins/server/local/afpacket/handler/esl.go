package handler

import (
	"github.com/google/gopacket"
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"
)

const (
	Protocol = "ESL"
	Name     = "ESL"
	ShowName = "ELS Handler"
)

type eslHandler struct {
	stats HandlerStats
}

func NewESLHandler() PacketHandler {
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

func (e *eslHandler) Stats() HandlerStats {
	return e.stats
}

func (e *eslHandler) Handle(packet gopacket.Packet) ([]*v1.SniffData, error) {
	return nil, nil
}
