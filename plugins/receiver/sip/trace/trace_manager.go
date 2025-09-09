package trace

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/sniffdata"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"
)

var PREFIX = "SNIFFER-"

type TraceContext struct {
	traceID   string
	idMapping []string // 使用dialog ID和transaction ID追踪spanID，value是dialog ID或transaction ID，index是spanID
	segment   *agent.SegmentObject

	isInitalized bool // 是否已经初始化
}

func (ctx *TraceContext) addMsgToSpan(id string, msgs []types.SipMessage) {
	if len(msgs) == 0 {
		return
	}
	spanId := slices.Index(ctx.idMapping, id)
	if spanId == -1 {
		log.Logger.Errorf("span with ID %s not found in trace context %s, cannot add messages", id, ctx.traceID)
		return
	}
	span := ctx.segment.Spans[spanId]
	span.Tags = append(span.Tags, buildSequenceTag(msgs))
}

func buildSequenceTag(msgs []types.SipMessage) *common.KeyStringValuePair {
	items := make([]*sniffdata.SipSequenceData, 0, len(msgs))
	for _, msg := range msgs {
		items = append(items, sniffdata.NewSipSequenceData(msg))
	}
	jsonStr, err := json.Marshal(items)
	if err != nil {
		log.Logger.Errorf("failed to marshal SipSequenceData: %v", err)
		jsonStr = []byte("[]")
	}
	return &common.KeyStringValuePair{
		Key:   "sip_seq_data",
		Value: string(jsonStr),
	}
}

type TraceManager struct {
	serviceName       string
	serviceInstanceId string
	traceContext      *sync.Map // key: trace ID
}

func (m *TraceManager) RemoveTraceContextByCallID(id string) {
	m.traceContext.Delete(wrapWithPrefix(id))
}

func NewTraceManager(serviceName, serviceInstanceId string) *TraceManager {
	return &TraceManager{
		serviceName:       serviceName,
		serviceInstanceId: serviceInstanceId,
		traceContext:      &sync.Map{},
	}
}

func (m *TraceManager) GetTraceContextByTraceID(traceID string) (*TraceContext, bool) {
	traceID = wrapWithPrefix(traceID)
	ctx, exists := m.traceContext.Load(traceID)
	if !exists {
		return nil, false
	}
	return ctx.(*TraceContext), true
}

func (m *TraceManager) CreateTraceContext(traceID string, createAt int64) *TraceContext {
	traceID = wrapWithPrefix(traceID)
	ctx, exists := m.traceContext.Load(traceID)
	if !exists {
		// 创建新的TraceContext
		segment := sniffdata.NewSegmentBuilder(m.serviceName, m.serviceInstanceId).WithTraceId(traceID).WithTimestamp(createAt).Build()
		ctx = &TraceContext{
			traceID:      traceID,
			segment:      segment,
			idMapping:    make([]string, 0), // 初始化idMapping
			isInitalized: false,             // 初始状态为未初始化
		}
		m.traceContext.Store(traceID, ctx)
	}
	return ctx.(*TraceContext)
}

func (ctx *TraceContext) initSegmentObject(req types.SipRequest, uaType types.UAType) {
	ctx.segment.Spans = make([]*agent.SpanObject, 0)
	ctx.idMapping = make([]string, 0)
	ctx.isInitalized = true
}

func (ctx *TraceContext) CreateNewSpan(id, parent, method, remoteURI string, startTime int64, headers map[string]string) {
	if !ctx.isInitalized {
		// 快速失败，没有初始化的TraceContext无法创建新的Span
		log.Logger.Errorf("TraceContext not initialized, cannot create new span for ID: %s", id)
		return
	}
	spanID := len(ctx.idMapping)
	parentID := ctx.getParentSpanID(id)
	ctx.idMapping = append(ctx.idMapping, id)
	// 创建新的Span
	span := sniffdata.NewSpanBuilder().
		WithSpanId(int32(spanID)).
		WithParentSpanId(parentID).
		WithStartTime(startTime).
		WithOperationName(strings.ToUpper(method)).
		WithHeaders(headers).
		WithSpanType(agent.SpanType_Local).     //为了方便管理先全部设置为Local
		WithSpanLayer(agent.SpanLayer_Unknown). // 自定义场景在protobuf中未定义，统统为unknown
		WithPeer(remoteURI).                    // 使用remoteURI作为对端地址
		Build()
	ctx.segment.Spans = append(ctx.segment.Spans, span)
}

func (ctx *TraceContext) FinishExistSpan(id string, isError bool, endTime int64) {
	for i, record := range ctx.idMapping {
		if record == id {
			span := ctx.segment.Spans[i]
			span.EndTime = endTime
			span.IsError = isError
			log.Logger.Infof("Finished span with ID %s in trace context %s, from: %d to %d", id, ctx.traceID, span.StartTime, span.EndTime)
			return
		}
	}
	// TODO 讨论是不是预定义ErrNotFound然后用log.Logger.WithError(ErrNotFound).Errorf()比较好
	log.Logger.Errorf("span with ID %s not found in trace context %s", id, ctx.traceID)
}

// 使用Dialog ID从idMapping中寻找parentID
// 由于sip对话（dialog）> 事务（transaction），且事务不能嵌套事务，所以这里只有dialog可能为parent，
// 也可能没有dialogID，此时对应out-dialog会话，parent固定为-1
func (ctx *TraceContext) getParentSpanID(id string) int32 {
	return int32(slices.Index(ctx.idMapping, id)) // 找不到返回-1
}

func (ctx *TraceContext) sendSegment(channel chan *v1.SniffData) {
	if ctx.isInitalized && len(ctx.segment.Spans) > 1 {
		// 发送SegmentObject到channel
		data := sniffdata.WrapWithSniffData(ctx.segment)
		channel <- data
	} else {
		log.Logger.Warnf("TraceContext %s is not initialized or has no spans, skipping send", ctx.traceID)
	}
}

func wrapWithPrefix(s string) string {
	if strings.HasPrefix(s, PREFIX) {
		return s
	}
	return fmt.Sprintf("%s%s", PREFIX, s)
}

func extractWithoutPrefix(s string) string {
	return strings.TrimPrefix(s, PREFIX)
}
