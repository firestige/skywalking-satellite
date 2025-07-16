package sip

import (
	"strconv"

	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	module "github.com/apache/skywalking-satellite/internal/satellite/module/api"
	forwarder "github.com/apache/skywalking-satellite/plugins/forwarder/api"
	"github.com/apache/skywalking-satellite/plugins/forwarder/grpc/nativelog"
	"github.com/apache/skywalking-satellite/plugins/forwarder/grpc/nativetracing"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/ghettovoice/gosip/sip"
	"github.com/ghettovoice/gosip/sip/parser"
	"google.golang.org/protobuf/proto"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	logging "skywalking.apache.org/repo/goapi/collect/logging/v3"
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
	return ``
}

func (r *Receiver) RegisterHandler(server interface{}) {
	r.Server = server.(*packet.Server)
	r.OutputChannel = make(chan *v1.SniffData, 1000)
	r.Server.RegisterHandler("SIP", "sip", r.packetHandler)
}

func (r *Receiver) RegisterSyncInvoker(_ module.SyncInvoker) {
	// No sync invoker needed for SIP receiver
}

func (r *Receiver) packetHandler(data *types.RawFrameData) error {
	segment := buildSegment(data)
	traceByte, _ := proto.Marshal(segment)
	traceData := &v1.SniffData{
		Name:   "sip-capture",
		Type:   v1.SniffType_TracingType,
		Remote: true,
		Data: &v1.SniffData_Segment{
			Segment: traceByte,
		},
	}
	r.OutputChannel <- traceData
	packet := buildLogData(data)
	packetByte, _ := proto.Marshal(packet)
	logData := &v1.SniffData{
		Name:   "sip-log",
		Type:   v1.SniffType_Logging,
		Remote: true,
		Data: &v1.SniffData_LogList{
			LogList: &v1.BatchLogList{
				Logs: [][]byte{packetByte},
			},
		},
	}
	r.OutputChannel <- logData
	return nil
}

func buildSegment(source *types.RawFrameData) *agent.SegmentObject {
	// 创建适配器
	adapter := &LoggerAdapter{logger: log.Logger}

	msg, err := parser.NewPacketParser(adapter).ParseMessage(source.Data)
	if err != nil {
		log.Logger.Error("failed to parse SIP message:", err)
		return nil
	}
	traceId, ok := msg.CallID()
	if !ok {
		log.Logger.Error("SIP message does not contain Call-ID header")
		return nil
	}
	segmentId, ok := msg.CSeq()
	if !ok {
		log.Logger.Error("SIP message does not contain CSeq header")
		return nil
	}

	var operationName string
	var isError bool
	// 使用类型断言判断是请求还是响应
	if req, ok := msg.(sip.Request); ok {
		// 这是一个请求
		operationName = string(req.Method()) + " " + req.Recipient().String()
		isError = false // 请求通常不会标记为错误
	} else if resp, ok := msg.(sip.Response); ok {
		// 这是一个响应
		operationName = strconv.Itoa(int(resp.StatusCode())) + " " + resp.Reason()
		isError = resp.StatusCode() >= 400 // 响应状态码 >= 400 标记为错误
	} else {
		log.Logger.Error("unknown SIP message type")
		return nil
	}

	return &agent.SegmentObject{
		TraceSegmentId:  segmentId.Value(),
		TraceId:         traceId.Value(),
		Service:         "SIP Service",
		ServiceInstance: "SIP Instance",
		Spans: []*agent.SpanObject{
			{
				SpanId:        0,
				SpanType:      agent.SpanType_Entry,
				SpanLayer:     agent.SpanLayer_Unknown,
				OperationName: operationName,
				StartTime:     source.Timestamp,
				EndTime:       source.Timestamp + 1000, // 假设处理时间为1000毫秒
				Tags: []*common.KeyStringValuePair{
					{
						Key: "from",
						Value: func() string {
							if fromHeader, ok := msg.From(); ok && fromHeader != nil {
								return fromHeader.String()
							}
							return ""
						}(),
					},
				},
				IsError: isError,
			},
		},
	}
}

func buildLogData(source *types.RawFrameData) *logging.LogData {
	return &logging.LogData{
		Service:         "SIP Service",
		ServiceInstance: "SIP Instance",
		Timestamp:       source.Timestamp,
		Endpoint:        "SIP Endpoint",
		Body: &logging.LogDataBody{
			Content: &logging.LogDataBody_Text{
				Text: &logging.TextLog{
					Text: string(source.Data),
				},
			},
		},
		Tags: &logging.LogTags{
			Data: []*common.KeyStringValuePair{
				{
					Key:   "sip.protocol",
					Value: "SIP",
				},
				{
					Key:   "sip.direction",
					Value: source.Direction,
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
