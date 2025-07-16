package esl

import (
	"fmt"
	"strings"

	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	module "github.com/apache/skywalking-satellite/internal/satellite/module/api"
	forwarder "github.com/apache/skywalking-satellite/plugins/forwarder/api"
	"github.com/apache/skywalking-satellite/plugins/forwarder/grpc/nativelog"
	"github.com/apache/skywalking-satellite/plugins/forwarder/grpc/nativetracing"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"google.golang.org/protobuf/proto"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	logging "skywalking.apache.org/repo/goapi/collect/logging/v3"
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"
)

const (
	Name        = "esl-receiver"
	ShowName    = "ESL Packet Receiver"
	Description = "A receiver plugin for ESL (Event Socket Library) protocol, used to receive events from FreeSWITCH servers."
)

type Receiver struct {
	config.CommonFields

	OutputChannel chan *v1.SniffData
	Server        *packet.Server
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
	return `{
		"protocol": "esl",
		"port": 8021
	}`
}

func (r *Receiver) RegisterHandler(server interface{}) {
	r.Server = server.(*packet.Server)
	r.OutputChannel = make(chan *v1.SniffData, 1000)
	r.Server.RegisterHandler("esl", "esl", r.packetHandler)
}

func (r *Receiver) RegisterSyncInvoker(_ module.SyncInvoker) {
	// No sync invoker needed for ESL receiver
}

func (r *Receiver) packetHandler(data *types.RawFrameData) error {
	parser := &ESLParser{}
	event, err := parser.ParseMessage(data.Data)
	if err != nil {
		log.Logger.Error("failed to parse ESL message:", err)
		return nil
	}

	// Build segment for tracing
	segment := buildSegment(data, event)
	if segment != nil {
		traceByte, _ := proto.Marshal(segment)
		traceData := &v1.SniffData{
			Name:   "esl-capture",
			Type:   v1.SniffType_TracingType,
			Remote: true,
			Data: &v1.SniffData_Segment{
				Segment: traceByte,
			},
		}
		r.OutputChannel <- traceData
	}

	// Build log data
	logData := buildLogData(data, event)
	if logData != nil {
		logByte, _ := proto.Marshal(logData)
		logSniffData := &v1.SniffData{
			Name:   "esl-log",
			Type:   v1.SniffType_Logging,
			Remote: true,
			Data: &v1.SniffData_LogList{
				LogList: &v1.BatchLogList{
					Logs: [][]byte{logByte},
				},
			},
		}
		r.OutputChannel <- logSniffData
	}

	return nil
}

func buildSegment(source *types.RawFrameData, event *ESLEvent) *agent.SegmentObject {
	// Use Event-UUID as trace ID if available
	traceId := event.Headers["Event-UUID"]
	if traceId == "" {
		traceId = event.Headers["Unique-ID"]
	}
	if traceId == "" {
		traceId = event.Headers["Core-UUID"]
	}
	if traceId == "" {
		return nil
	}

	// Use Event-Name as operation name
	operationName := event.Headers["Event-Name"]
	if operationName == "" {
		operationName = "ESL_EVENT"
	}

	// Determine if this is an error based on event type
	isError := strings.Contains(strings.ToLower(operationName), "error") ||
		strings.Contains(strings.ToLower(operationName), "fail")

	return &agent.SegmentObject{
		TraceId:         traceId,
		TraceSegmentId:  traceId,
		Service:         "ESL Service",
		ServiceInstance: "ESL Instance",
		Spans: []*agent.SpanObject{
			{
				SpanId:        1,
				ParentSpanId:  0,
				StartTime:     source.Timestamp,
				EndTime:       source.Timestamp,
				OperationName: operationName,
				SpanType:      agent.SpanType_Entry,
				SpanLayer:     agent.SpanLayer_RPCFramework,
				IsError:       isError,
				Tags: []*common.KeyStringValuePair{
					{
						Key:   "esl.event_name",
						Value: operationName,
					},
					{
						Key:   "esl.caller_id",
						Value: event.Headers["Caller-Caller-ID-Number"],
					},
					{
						Key:   "esl.destination",
						Value: event.Headers["Caller-Destination-Number"],
					},
					{
						Key:   "esl.channel",
						Value: event.Headers["Channel-Name"],
					},
					{
						Key:   "esl.direction",
						Value: event.Headers["Call-Direction"],
					},
				},
			},
		},
	}
}

func buildLogData(source *types.RawFrameData, event *ESLEvent) *logging.LogData {
	eventName := event.Headers["Event-Name"]
	if eventName == "" {
		eventName = "UNKNOWN_EVENT"
	}

	return &logging.LogData{
		Timestamp:       source.Timestamp,
		Service:         "ESL Service",
		ServiceInstance: "ESL Instance",
		Body: &logging.LogDataBody{
			Type: "json",
			Content: &logging.LogDataBody_Json{
				Json: &logging.JSONLog{
					Json: fmt.Sprintf(`{
						"event_name": "%s",
						"caller_id": "%s",
						"destination": "%s",
						"channel": "%s",
						"direction": "%s",
						"body": "%s"
					}`,
						eventName,
						event.Headers["Caller-Caller-ID-Number"],
						event.Headers["Caller-Destination-Number"],
						event.Headers["Channel-Name"],
						event.Headers["Call-Direction"],
						strings.ReplaceAll(event.Body, `"`, `\"`),
					),
				},
			},
		},
		Tags: &logging.LogTags{
			Data: []*common.KeyStringValuePair{
				{
					Key:   "event_name",
					Value: eventName,
				},
				{
					Key:   "caller_id",
					Value: event.Headers["Caller-Caller-ID-Number"],
				},
				{
					Key:   "unique_id",
					Value: event.Headers["Unique-ID"],
				},
			},
		},
	}
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
