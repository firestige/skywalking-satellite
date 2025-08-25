package sip

import (
	"fmt"

	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	module "github.com/apache/skywalking-satellite/internal/satellite/module/api"
	forwarder "github.com/apache/skywalking-satellite/plugins/forwarder/api"
	"github.com/apache/skywalking-satellite/plugins/forwarder/grpc/nativelog"
	"github.com/apache/skywalking-satellite/plugins/forwarder/grpc/nativetracing"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/session"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/trace"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet"
	"github.com/google/gopacket/layers"
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"
)

const (
	Name        = "sip-receiver"
	ShowName    = "SIP Packet Receiver"
	Description = "A receiver plugin for SIP (Session Initiation Protocol), used to receive events from SIP servers."
)

type Receiver struct {
	config.CommonFields
	ServiceName     string `mapstructure:"service_name"`     // 服务名称
	ServiceInstance string `mapstructure:"service_instance"` // 服务实例
	LocalIp         string `mapstructure:"local_ip"`         // 本地IP地址，接收SIP消息的IP地址
	Ports           string `mapstructure:"ports"`            // 监听的端口列表，逗号分隔

	OutputChannel chan *v1.SniffData
	Server        *packet.Server
	sipParser     *SipParser
	handler       *session.SessionHandler
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
	return `
service_name: "SIP Service"
service_instance: "SIP Instance"
local_ip: "127.0.0.1"
ports: "5060,5061" # 监听的端口列表，逗号分隔
`
}

func (r *Receiver) RegisterHandler(server interface{}) {
	r.Server = server.(*packet.Server)
	r.OutputChannel = make(chan *v1.SniffData, 10000)
	r.sipParser = NewSipParser()
	r.handler = session.NewSessionHandler()
	submit := func(data *v1.SniffData) {
		log.Logger.Debugf("Submitting data: %s", data.Name)
		r.OutputChannel <- data
	}
	r.handler.RegisterListener(trace.NewTraceListener(r.ServiceName, r.ServiceInstance, submit))
	log.Logger.Infof("SIP Receiver initialized with service name: %s, instance: %s, ports: %s", r.ServiceName, r.ServiceInstance, r.Ports)
	r.Server.RegisterHandler(layers.IPProtocolTCP, r.Ports, fmt.Sprintf("%s-TCP", r.ServiceName), r.processTCPFrame)
	r.Server.RegisterHandler(layers.IPProtocolUDP, r.Ports, fmt.Sprintf("%s-UDP", r.ServiceName), r.processUDPFrame)
}

func (r *Receiver) RegisterSyncInvoker(_ module.SyncInvoker) {
	// No sync invoker needed for SIP receiver
}

func (r *Receiver) Channel() <-chan *v1.SniffData {
	return r.OutputChannel
}

func (r *Receiver) SupportForwarders() []forwarder.Forwarder {
	return []forwarder.Forwarder{
		new(nativelog.Forwarder),
		new(nativetracing.Forwarder),
	}
}

// Stop 停止接收器时清理资源
func (r *Receiver) Stop() {
}
