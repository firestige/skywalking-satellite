package packet

import (
	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/plugins/server/api"
)

type serverAdapter struct {
	config.CommonFields
}

func NewServer() api.Server {
	return &serverAdapter{}
}

func (s *serverAdapter) Name() string {
	return "packet-server"
}

func (s *serverAdapter) ShowName() string {
	return "Packet Server"
}

func (s *serverAdapter) Description() string {
	return "A server plugin for packet capture and processing"
}

func (s *serverAdapter) DefaultConfig() string {
	return ``
}

func (s *serverAdapter) GetServer() interface{} {
	return s
}

func (s *serverAdapter) Prepare() error {
	// Preparation logic for the packet server
	// This could include setting up network interfaces, buffers, etc.
	return nil
}

func (s *serverAdapter) Start() error {
	return nil
}

func (s *serverAdapter) Close() error {
	return nil
}
