package netcapture

import (
	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"google.golang.org/protobuf/proto"

	module "github.com/apache/skywalking-satellite/internal/satellite/module/api"
	v3 "skywalking.apache.org/repo/goapi/collect/logging/v3"
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"

	forwarder "github.com/apache/skywalking-satellite/plugins/forwarder/api"
	"github.com/apache/skywalking-satellite/plugins/forwarder/grpc/nativelog"

	"github.com/apache/skywalking-satellite/plugins/server/afpacket"
)

const (
	// Name is the name of the NetCapture receiver.
	Name     = "net-capture-receiver"
	ShowName = "Net Packet Capture Receiver"
)

type Receiver struct {
	config.CommonFields
	// OutputChannel is the channel where captured packets will be sent.
	OutputChannel chan *v1.SniffData
	// Server is the AFPacket server that captures packets from the network interface.
	Server *afpacket.Server
}

func (r *Receiver) Name() string {
	return Name
}

func (r *Receiver) ShowName() string {
	return ShowName
}

func (r *Receiver) Description() string {
	return "NetCapture Receiver captures network packets from a specified interface and processes them for analysis."
}

func (r *Receiver) DefaultConfig() string {
	return ``
}

func (r *Receiver) RegisterHandler(server interface{}) {
	r.Server = server.(*afpacket.Server)
	r.OutputChannel = make(chan *v1.SniffData)
	r.Server.EventLoop.RegisterHandler(r.packetHandler())
}

func (r *Receiver) RegisterSyncInvoker(_ module.SyncInvoker) {
}

func (r *Receiver) packetHandler() func(event *afpacket.Event) error {
	return func(event *afpacket.Event) error {
		log.Logger.Printf("Received event: %s", event.Name)
		if event == nil || event.Payload == nil {
			log.Logger.Info("Received nil packet, ignoring")
			return nil // Ignore nil packets
		}

		msg := &v3.LogData{
			Timestamp: event.Timestamp,
			Service:   "sip-uac-message",
			Body: &v3.LogDataBody{
				Type: "LogDataBody_Text",
				Content: &v3.LogDataBody_Text{
					Text: &v3.TextLog{
						Text: string(event.Payload),
					},
				},
			},
			TraceContext: &v3.TraceContext{},
			Tags:         &v3.LogTags{},
		}
		msgByte, err := proto.Marshal(msg)
		if err != nil {
			log.Logger.Warn("packet marshal failure")
		}
		// Create a SniffData event from the packet
		sniffData := &v1.SniffData{
			Name:      "net-capture-event",
			Timestamp: event.Timestamp,
			Type:      v1.SniffType_Logging,
			Remote:    true,
			Data: &v1.SniffData_LogList{
				LogList: &v1.BatchLogList{
					Logs: [][]byte{msgByte},
				},
			},
		}
		// log.Logger.Printf("Captured packet: %s", event.Payload) // Log the first 32 bytes of the packet
		r.OutputChannel <- sniffData
		return nil // Return nil to indicate successful processing
	}
}

func (r *Receiver) Channel() <-chan *v1.SniffData {
	return r.OutputChannel
}

func (r *Receiver) SupportForwarders() []forwarder.Forwarder {
	return []forwarder.Forwarder{
		// new(nativetracing.Forwarder),
		new(nativelog.Forwarder),
		// new(nativemeter.Forwarder),
	}
}
