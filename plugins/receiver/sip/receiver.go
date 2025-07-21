package sip

import (
	"strconv"
	"time"

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
	Name        = "sip-receiver"
	ShowName    = "SIP Packet Receiver"
	Description = "A receiver plugin for SIP (Session Initiation Protocol), used to receive events from SIP servers."
)

type Receiver struct {
	config.CommonFields
	ServiceName     string `mapstructure:"service_name"`     // 服务名称
	ServiceInstance string `mapstructure:"service_instance"` // 服务实例

	OutputChannel  chan *v1.SniffData
	Server         *packet.Server
	sipParser      *SipParser
	sessionManager SipSessionManager
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
	`
}

func (r *Receiver) RegisterHandler(server interface{}) {
	r.Server = server.(*packet.Server)
	r.OutputChannel = make(chan *v1.SniffData, 1000)
	r.sipParser = NewSipParser()
	config := &SessionManagerConfig{
		ServiceName:     r.ServiceName,
		ServiceInstance: r.ServiceInstance,

		SessionTTL:      5 * time.Minute, // 会话过期时间
		CleanupInterval: 1 * time.Minute, // 自动清理间隔
	}
	r.sessionManager = NewSessionManager(*config)
	r.Server.RegisterHandler("UDP", "sip", r.packetHandler)
}

func (r *Receiver) RegisterSyncInvoker(_ module.SyncInvoker) {
	// No sync invoker needed for SIP receiver
}

func (r *Receiver) packetHandler(data *types.RawFrameData) error {
	// 解析SIP消息
	sipMessage, err := r.sipParser.Parse(data.Data)
	if err != nil {
		log.Logger.Error("failed to parse SIP message:", err)
		return err
	}

	session, err := r.sessionManager.GetOrCreateSession(sipMessage)

	// 构建跟踪段
	segment := r.buildSegment(data, sipMessage, session)
	if segment == nil {
		log.Logger.Error("failed to build segment")
		return err
	}

	// 发送跟踪数据
	traceByte, _ := proto.Marshal(segment)
	traceData := &v1.SniffData{
		Name:      "sip-capture",
		Timestamp: data.Timestamp,
		Type:      v1.SniffType_TracingType,
		Remote:    true,
		Data: &v1.SniffData_Segment{
			Segment: traceByte,
		},
	}
	r.OutputChannel <- traceData

	// 构建并发送日志数据
	packet := buildLogData(data, sipMessage, session)
	packetByte, _ := proto.Marshal(packet)
	logData := &v1.SniffData{
		Name:      "sip-log",
		Timestamp: data.Timestamp,
		Type:      v1.SniffType_Logging,
		Remote:    true,
		Data: &v1.SniffData_LogList{
			LogList: &v1.BatchLogList{
				Logs: [][]byte{packetByte},
			},
		},
	}
	r.OutputChannel <- logData
	return nil
}

// buildSegment 根据SIP消息构建跟踪段
// SegmentObjectd的定义如下
//
//	{
//	  TraceSegmentId string // 跟踪段ID,已经在session的segment中定义,不用修改
//	  TraceId string // 跟踪ID,已经在session的segment中定义,不用修改
//	  Service string // 服务名称,从配置文件中读取,在receiver的config中新增定义
//	  ServiceInstance string // 服务实例名称,从配置文件中读取,在receiver的config中新增定义
//	  Spans []*SpanObject // Span对象列表,已经在session的segment中定义,按照session中定义的currentSpan获取接下来要填写的span对象
//	}
//
// SpanObject的定义如下
//
//	{
//	  SpanId int32 // Span ID,已经在session的segment中定义,不用修改
//	  ParentSpanId int32 // 父Span ID,这里默认所有的Span都是根Span,所以ParentSpanId为-1
//	  StartTime int64 // Span开始时间,如果sipMessage是Request, 则从RawFrameData的Timestamp中获取,否则不填
//	  EndTime int64 // Span结束时间,如果sipMessage是Response, 则从RawFrameData的Timestamp中获取,否则不填
//	  OperationName string // 操作名称,根据sipMessage的类型构建,仅在没有值且sipMessage是Request时填入RequestLine,其他时候不填
//	  SpanType SpanType // 这里仅在sipMessage是Request时处理, 如果sipMessage是inbound request, 则SpanType为Entry, outbound request为Exit, 否则为Local
//	  SpanLayer SpanLayer // 这里默认是Unknown,因为SIP协议没有明确的层级
//	  ComponentId int32 // 组件ID,这里默认是0,因为SIP协议没有明确的组件ID
//	  Peer string // 对端地址,如果sipMessage是Request, 则从RawFrameData的RemoteAddress中获取, 否则不填
//	  Tags []*KeyStringValuePair // 标签列表,这里将sipMessage的Headers转换为标签,如果sipMessage是Request, 则将Method作为标签
//	  IsError bool // 是否为错误,如果sipMessage是Response且状态码大于等于400, 则为true, 否则为false
//	}
func (r *Receiver) buildSegment(source *types.RawFrameData, sipMessage SipMessage, session *Session) *agent.SegmentObject {
	// 获取 session 中已经维护好的 segment 对象
	segment := session.Segment

	// 获取当前需要处理的 span（按照 session 中的 CurrentSpan 索引）
	if int(session.CurrentSpan) >= len(segment.Spans) {
		log.Logger.Errorf("CurrentSpan index %d out of range for spans length %d", session.CurrentSpan, len(segment.Spans))
		return segment
	}

	currentSpan := segment.Spans[session.CurrentSpan]

	// 根据消息类型就地修改 span 对象
	if sipMessage.IsRquest() {
		// 处理请求消息
		if req, ok := sipMessage.(SipRequest); ok {
			// 设置 span 开始时间
			currentSpan.StartTime = source.Timestamp

			// 设置操作名称（仅在没有值时填入）
			if currentSpan.OperationName == "" {
				currentSpan.OperationName = source.Direction + req.RequestLine()
			}

			// 设置 SpanType（判断是 inbound 还是 outbound request）
			// 这里简化处理，可以根据实际需求调整判断逻辑
			switch source.Direction {
			case "inbound":
				currentSpan.SpanType = agent.SpanType_Entry
			case "outbound":
				currentSpan.SpanType = agent.SpanType_Exit
			default:
				currentSpan.SpanType = agent.SpanType_Local
			}

			// 设置 SpanLayer
			currentSpan.SpanLayer = agent.SpanLayer_Unknown

			// 设置组件ID
			currentSpan.ComponentId = 0

			// 设置对端地址
			currentSpan.Peer = source.Connection.SrcHost + ":" + strconv.Itoa(source.Connection.SrcPort)

			// 设置标签
			tags := make([]*common.KeyStringValuePair, 0)

			// 将 Method 作为标签添加
			tags = append(tags, &common.KeyStringValuePair{
				Key:   "sip.method",
				Value: req.Method(),
			})

			// 将 Headers 转换为标签
			for key, value := range sipMessage.Headers() {
				tags = append(tags, &common.KeyStringValuePair{
					Key:   "sip.header." + key,
					Value: value,
				})
			}

			currentSpan.Tags = tags
			currentSpan.IsError = false
		}
	} else {
		// 处理响应消息
		if resp, ok := sipMessage.(SipResponse); ok {
			// 设置 span 结束时间
			currentSpan.EndTime = source.Timestamp

			// 设置错误状态
			currentSpan.IsError = resp.Status() >= 400

			// 添加响应相关的标签
			if currentSpan.Tags == nil {
				currentSpan.Tags = make([]*common.KeyStringValuePair, 0)
			}

			currentSpan.Tags = append(currentSpan.Tags, &common.KeyStringValuePair{
				Key:   "sip.status_code",
				Value: strconv.Itoa(resp.Status()),
			})

			currentSpan.Tags = append(currentSpan.Tags, &common.KeyStringValuePair{
				Key:   "sip.status_line",
				Value: resp.StatusLine(),
			})

			// 处理响应后，将 CurrentSpan 减 1，指向之前一个请求的 span
			if session.CurrentSpan > 0 {
				session.CurrentSpan--
			}
		}
	}

	return segment
}

func buildLogData(source *types.RawFrameData, message SipMessage, session *Session) *logging.LogData {
	return &logging.LogData{
		Service:         "SIP Service",
		ServiceInstance: "SIP Instance",
		Timestamp:       source.Timestamp,
		Endpoint:        "SIP Endpoint",
		Body: &logging.LogDataBody{
			Type: "LogDataBodyType_TEXT",
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
		TraceContext: &logging.TraceContext{
			TraceId:        session.Segment.TraceId,
			TraceSegmentId: session.Segment.TraceSegmentId,
			SpanId:         session.CurrentSpan,
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

// Stop 停止接收器时清理资源
func (r *Receiver) Stop() {
}
