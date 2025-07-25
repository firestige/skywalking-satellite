package sniffdata

import (
	"google.golang.org/protobuf/proto"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"
)

type SegmentBuilder struct {
	ServiceName     string
	ServiceInstance string
	Timestamp       int64
	TraceId         string
	SegmentId       string
	Spans           []*agent.SpanObject
}

func NewSegmentBuilder() *SegmentBuilder {
	return &SegmentBuilder{
		Spans: make([]*agent.SpanObject, 0),
	}
}

func (b *SegmentBuilder) WithServiceName(serviceName string) *SegmentBuilder {
	// 设置服务名称
	b.ServiceName = serviceName
	return b
}

func (b *SegmentBuilder) WithServiceInstance(serviceInstance string) *SegmentBuilder {
	// 设置服务实例名称
	b.ServiceInstance = serviceInstance
	return b
}

func (b *SegmentBuilder) WithTimestamp(timestamp int64) *SegmentBuilder {
	// 设置时间戳
	b.Timestamp = timestamp
	return b
}

func (b *SegmentBuilder) WithTraceId(traceId string) *SegmentBuilder {
	// 设置跟踪ID
	b.TraceId = traceId
	return b
}

func (b *SegmentBuilder) WithSegmentId(segmentId string) *SegmentBuilder {
	// 设置段ID
	b.SegmentId = segmentId
	return b
}

func (b *SegmentBuilder) WithSpan(span *agent.SpanObject) *SegmentBuilder {
	// 添加Span到段中
	b.Spans = append(b.Spans, span)
	return b
}

func (b *SegmentBuilder) WithSpans(spans []*agent.SpanObject) *SegmentBuilder {
	// 设置多个Span
	b.Spans = spans
	return b
}

func (b *SegmentBuilder) WithSpanBuilder() *SpanBuilder {
	// 添加span到段中
	return newSpanBuilder(b)
}

func (b *SegmentBuilder) buildSegment() *agent.SegmentObject {
	// 构建跟踪段对象
	return &agent.SegmentObject{
		TraceId:         b.TraceId,
		TraceSegmentId:  b.SegmentId,
		Spans:           []*agent.SpanObject{},
		Service:         b.ServiceName,
		ServiceInstance: b.ServiceInstance,
		IsSizeLimited:   false, // 先不考虑截断span提升传输吞吐量
	}
}

func (b *SegmentBuilder) Build() *v1.SniffData {
	segment := b.buildSegment()
	traceByte, _ := proto.Marshal(segment)
	return &v1.SniffData{
		Name:      "sip-capture",
		Timestamp: b.Timestamp,
		Type:      v1.SniffType_TracingType,
		Remote:    true,
		Data: &v1.SniffData_Segment{
			Segment: traceByte,
		},
	}
}

type SpanBuilder struct {
	SpanId        int32
	ParentSpanId  int32
	StartTime     int64
	EndTime       int64
	Refs          []*agent.SegmentReference
	OperationName string
	Peer          string
	SpanType      agent.SpanType
	SpanLayer     agent.SpanLayer
	ComponentId   int32
	IsError       bool
	Tags          []*common.KeyStringValuePair
	Logs          []*agent.Log
	SkipAnalysis  bool
	parent        *SegmentBuilder
}

func newSpanBuilder(parentBuilder *SegmentBuilder) *SpanBuilder {
	return &SpanBuilder{
		Refs:   make([]*agent.SegmentReference, 0),
		Tags:   make([]*common.KeyStringValuePair, 0),
		Logs:   make([]*agent.Log, 0),
		parent: parentBuilder,
	}
}

func (b *SpanBuilder) WithSpanId(spanId int32) *SpanBuilder {
	// 设置Span ID
	b.SpanId = spanId
	return b
}

func (b *SpanBuilder) WithParentSpanId(parentSpanId int32) *SpanBuilder {
	// 设置父Span ID
	b.ParentSpanId = parentSpanId
	return b
}

func (b *SpanBuilder) WithStartTime(startTime int64) *SpanBuilder {
	// 设置Span开始时间
	b.StartTime = startTime
	return b
}

func (b *SpanBuilder) WithEndTime(endTime int64) *SpanBuilder {
	// 设置Span结束时间
	b.EndTime = endTime
	return b
}

func (b *SpanBuilder) WithRef(refs *agent.SegmentReference) *SpanBuilder {
	// 设置Span引用
	b.Refs = append(b.Refs, refs)
	return b
}

func (b *SpanBuilder) WithOperationName(operationName string) *SpanBuilder {
	// 设置操作名称
	b.OperationName = operationName
	return b
}

func (b *SpanBuilder) WithPeer(peer string) *SpanBuilder {
	// 设置对端地址
	b.Peer = peer
	return b
}

func (b *SpanBuilder) WithSpanType(spanType agent.SpanType) *SpanBuilder {
	// 设置Span类型
	b.SpanType = spanType
	return b
}

func (b *SpanBuilder) WithSpanLayer(spanLayer agent.SpanLayer) *SpanBuilder {
	// 设置Span层级
	b.SpanLayer = spanLayer
	return b
}

func (b *SpanBuilder) WithComponentId(componentId int32) *SpanBuilder {
	// 设置组件ID
	b.ComponentId = componentId
	return b
}

func (b *SpanBuilder) WithIsError(isError bool) *SpanBuilder {
	// 设置是否为错误Span
	b.IsError = isError
	return b
}

func (b *SpanBuilder) WithTag(key string, value string) *SpanBuilder {
	// 设置单个标签
	b.Tags = append(b.Tags, &common.KeyStringValuePair{
		Key:   key,
		Value: value,
	})
	return b
}

func (b *SpanBuilder) WithTags(tags []*common.KeyStringValuePair) *SpanBuilder {
	// 设置标签
	b.Tags = tags
	return b
}

func (b *SpanBuilder) WithLog(timestamp int64, metrices map[string]string) *SpanBuilder {
	// 设置单个日志
	log := &agent.Log{
		Time: timestamp,
		Data: make([]*common.KeyStringValuePair, 0),
	}
	for key, value := range metrices {
		log.Data = append(log.Data, &common.KeyStringValuePair{
			Key:   key,
			Value: value,
		})
	}
	b.Logs = append(b.Logs, log)
	return b
}

func (b *SpanBuilder) WithLogs(logs []*agent.Log) *SpanBuilder {
	// 设置日志
	b.Logs = logs
	return b
}

func (b *SpanBuilder) WithSkipAnalysis(skip bool) *SpanBuilder {
	// 设置是否跳过分析
	b.SkipAnalysis = skip
	return b
}

func (b *SpanBuilder) Build() *agent.SpanObject {
	// 构建Span对象
	return &agent.SpanObject{
		SpanId:        b.SpanId,
		ParentSpanId:  b.ParentSpanId,
		StartTime:     b.StartTime,
		EndTime:       b.EndTime,
		Refs:          b.Refs,
		OperationName: b.OperationName,
		Peer:          b.Peer,
		SpanType:      b.SpanType,
		SpanLayer:     b.SpanLayer,
		ComponentId:   b.ComponentId,
		IsError:       b.IsError,
		Tags:          b.Tags,
		Logs:          b.Logs,
		SkipAnalysis:  b.SkipAnalysis,
	}
}

func (b *SpanBuilder) reset() {
	// 重置SpanBuilder状态
	b.SpanId = 0
	b.ParentSpanId = 0
	b.StartTime = 0
	b.EndTime = 0
	b.Refs = make([]*agent.SegmentReference, 0)
	b.OperationName = ""
	b.Peer = ""
	b.SpanType = agent.SpanType_Local
	b.SpanLayer = agent.SpanLayer_Unknown
	b.ComponentId = 0
	b.IsError = false
	b.Tags = make([]*common.KeyStringValuePair, 0)
	b.Logs = make([]*agent.Log, 0)
	b.SkipAnalysis = false
}

func (b *SpanBuilder) And() *SpanBuilder {
	b.parent.Spans = append(b.parent.Spans, b.Build())
	// 清空当前SpanBuilder的状态以便下次使用
	b.reset()
	// 返回父SegmentBuilder
	return b
}

func (b *SpanBuilder) Finish() *SegmentBuilder {
	b.parent.Spans = append(b.parent.Spans, b.Build())
	// 清空当前SpanBuilder的状态以便下次使用
	// 返回父SegmentBuilder
	return b.parent
}

type SegmentReferenceBuilder struct {
	TraceId                  string
	ParentTraceSegmentId     string
	ParentSpanId             int32
	ParentService            string
	ParentServiceInstance    string
	ParentEndpoint           string
	NetworkAddressUsedAtPeer string
}

func NewSegmentReferenceBuilder() *SegmentReferenceBuilder {
	return &SegmentReferenceBuilder{}
}

func (b *SegmentReferenceBuilder) WithTraceId(traceId string) *SegmentReferenceBuilder {
	// 设置TraceId
	b.TraceId = traceId
	return b
}

func (b *SegmentReferenceBuilder) WithParentTraceSegmentId(parentTraceSegmentId string) *SegmentReferenceBuilder {
	// 设置父TraceSegmentId
	b.ParentTraceSegmentId = parentTraceSegmentId
	return b
}

func (b *SegmentReferenceBuilder) WithParentSpanId(parentSpanId int32) *SegmentReferenceBuilder {
	// 设置父SpanId
	b.ParentSpanId = parentSpanId
	return b
}

func (b *SegmentReferenceBuilder) WithParentService(parentService string) *SegmentReferenceBuilder {
	// 设置父服务名称
	b.ParentService = parentService
	return b
}

func (b *SegmentReferenceBuilder) WithParentServiceInstance(parentServiceInstance string) *SegmentReferenceBuilder {
	// 设置父服务实例名称
	b.ParentServiceInstance = parentServiceInstance
	return b
}

func (b *SegmentReferenceBuilder) WithParentEndpoint(parentEndpoint string) *SegmentReferenceBuilder {
	// 设置父端点
	b.ParentEndpoint = parentEndpoint
	return b
}

func (b *SegmentReferenceBuilder) WithNetworkAddressUsedAtPeer(networkAddressUsedAtPeer string) *SegmentReferenceBuilder {
	// 设置网络地址
	b.NetworkAddressUsedAtPeer = networkAddressUsedAtPeer
	return b
}

func (b *SegmentReferenceBuilder) Build() *agent.SegmentReference {
	// 构建SegmentReference对象
	return &agent.SegmentReference{
		RefType:                  agent.RefType_CrossProcess,
		TraceId:                  b.TraceId,
		ParentTraceSegmentId:     b.ParentTraceSegmentId,
		ParentSpanId:             b.ParentSpanId,
		ParentService:            b.ParentService,
		ParentServiceInstance:    b.ParentServiceInstance,
		ParentEndpoint:           b.ParentEndpoint,
		NetworkAddressUsedAtPeer: b.NetworkAddressUsedAtPeer,
	}
}
