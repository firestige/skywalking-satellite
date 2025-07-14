package sip

import (
	"github.com/apache/skywalking-satellite/internal/pkg/config"
	module "github.com/apache/skywalking-satellite/internal/satellite/module/api"
	forwarder "github.com/apache/skywalking-satellite/plugins/forwarder/api"
	"github.com/apache/skywalking-satellite/plugins/forwarder/grpc/nativelog"
	"github.com/apache/skywalking-satellite/plugins/forwarder/grpc/nativemeter"
	"github.com/apache/skywalking-satellite/plugins/forwarder/grpc/nativetracing"
	"github.com/apache/skywalking-satellite/plugins/server/local/afpacket"
	"github.com/apache/skywalking-satellite/plugins/server/local/afpacket/types"
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"
)

const (
	Name        = "sip-receiver"
	ShowName    = "SIP Packet Receiver"
	Description = "A receiver plugin for SIP (Session Initiation Protocol), used to receive events from SIP servers."
)

type Receiver struct {
	config.CommonFields

	OutputChannel chan *v1.SniffData
	Server        *afpacket.Server
}

func (r *Receiver) Name() string {
	return Name
}

func (r *Receiver) ShowName() string {
	return ShowName
}

func (r *Receiver) Description() string {
	return Description
}

func (r *Receiver) DefaultConfig() string {
	return ``
}

func (r *Receiver) RegisterHandler(server interface{}) {
	r.Server = server.(*afpacket.Server)
	r.OutputChannel = make(chan *v1.SniffData, 1000)
	r.Server.RegisterHandler("esl", r.packetHandler)
}

func (r *Receiver) RegisterSyncInvoker(_ module.SyncInvoker) {
	// No sync invoker needed for ESL receiver
}

func (r *Receiver) packetHandler(*types.RawFrameData) error {
	return nil // Implement the logic to handle ESL packets here
}

func (r *Receiver) Channel() <-chan *v1.SniffData {
	return r.OutputChannel
}

func (r *Receiver) SupportForwarders() []forwarder.Forwarder {
	return []forwarder.Forwarder{
		new(nativelog.Forwarder),
		new(nativetracing.Forwarder),
		new(nativemeter.Forwarder),
	}
}
