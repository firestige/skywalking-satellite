package trace

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/sniffdata"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/utils"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"
)

var PREFIX = "SNIFFER-"

type TraceContext struct {
	traceID string
	txID    string
	segment *agent.SegmentObject

	isInitalized bool // 是否已经初始化
	isProxyNode  bool // 是否是调用链的转发节点
}

func (ctx *TraceContext) addMsgToSpan(msgs []types.SipMessage) {
	if len(msgs) == 0 {
		return
	}
	span := ctx.segment.Spans[0]
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

func (m *TraceManager) RemoveTraceContextByTransactionID(id string) {
	m.traceContext.Delete(id)
}

func NewTraceManager(serviceName, serviceInstanceId string) *TraceManager {
	m := &TraceManager{
		serviceName:       serviceName,
		serviceInstanceId: serviceInstanceId,
		traceContext:      &sync.Map{},
	}
	go func() {
		for {
			m.printMetrics()
			time.Sleep(time.Minute)
		}
	}()
	return m
}

func (m *TraceManager) printMetrics() {
	length := 0
	m.traceContext.Range(func(_, _ interface{}) bool {
		length++
		return true
	})
	log.Logger.Infof("current trace context map size: %d", length)
}

func (m *TraceManager) GetTraceContextByTransactionID(txID string) (*TraceContext, bool) {
	ctx, exists := m.traceContext.Load(txID)
	if !exists {
		return nil, false
	}
	return ctx.(*TraceContext), true
}

func (m *TraceManager) CreateTraceContext(txID string, createAt int64) *TraceContext {
	traceID := wrapWithPrefix(txID)
	ctx, exists := m.traceContext.Load(txID)
	if !exists {
		// 创建新的TraceContext

		segment := sniffdata.NewSegmentBuilder(m.serviceName, m.serviceInstanceId).WithTraceId(traceID).WithTimestamp(createAt).Build()
		// segment.TraceSegmentId = renewSegmentID(txID)
		ctx = &TraceContext{
			traceID:      traceID,
			segment:      segment,
			txID:         txID,
			isInitalized: false, // 初始状态为未初始化
		}
		m.traceContext.Store(txID, ctx)
	}
	return ctx.(*TraceContext)
}

func (ctx *TraceContext) initSegmentObject() {
	ctx.segment.Spans = make([]*agent.SpanObject, 0)
	ctx.isInitalized = true
}

func (ctx *TraceContext) CreateNewSpan(id, method, remoteURI string, startTime int64, headers map[string]string, shouldUpdateRef bool, ref *agent.SegmentReference) {
	if !ctx.isInitalized {
		// 快速失败，没有初始化的TraceContext无法创建新的Span
		log.Logger.Errorf("TraceContext not initialized, cannot create new span for ID: %s", id)
		return
	}
	component := getComponentIdFromServiceName(ctx.segment.Service)
	builder := sniffdata.NewSpanBuilder().
		WithSpanId(0).
		WithParentSpanId(-1).
		WithStartTime(startTime).
		WithOperationName(strings.ToUpper(method)).
		WithHeaders(headers).
		WithSpanType(agent.SpanType_Entry).     //为了方便管理先全部设置为Entry
		WithSpanLayer(agent.SpanLayer_Unknown). // 自定义场景在protobuf中未定义，统统为unknown
		WithPeer(remoteURI).                    // 使用remoteURI作为对端地址
		WithComponentId(component)
	if ref != nil {
		builder = builder.WithRef(ref)
	}
	var span *agent.SpanObject
	if shouldUpdateRef {
		span = builder.WithTag("SHOULD_UPDATE_REF", "TRUE").Build()
	} else {
		span = builder.Build()
	}
	ctx.segment.Spans = append(ctx.segment.Spans, span)
}

func getComponentIdFromServiceName(s string) int32 {
	if strings.Contains(strings.ToUpper(s), "FREESWITCH") {
		return 5600
	}
	if strings.Contains(strings.ToUpper(s), "KAMAILIO") {
		return 5601
	}
	if strings.Contains(strings.ToUpper(s), "SBC") {
		return 5602
	}
	return 0
}

func (ctx *TraceContext) FinishExistSpan(id string, isError bool, endTime int64) {
	if ctx.txID == id {
		span := ctx.segment.Spans[0]
		span.EndTime = endTime
		span.IsError = isError
		log.Logger.Infof("Finished span with ID %s in trace context %s, from: %d to %d", id, ctx.traceID, span.StartTime, span.EndTime)
		return
	}
	// TODO 讨论是不是预定义ErrNotFound然后用log.Logger.WithError(ErrNotFound).Errorf()比较好
	log.Logger.Errorf("span with ID %s not found in trace context %s", id, ctx.traceID)
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

// 处理 traceID，一般是从 transactionID 转换而来，取｜的前半部分，并添加前缀 SNIFFER-
func wrapWithPrefix(s string) string {
	if strings.HasPrefix(s, PREFIX) {
		return s
	}
	idx := strings.Index(s, "|")
	if idx < 0 {
		return s
	}
	// 如果包含|，只对|前面的部分添加前缀
	return fmt.Sprintf("%s%s", PREFIX, s[:idx])
}

func extractWithoutPrefix(s string) string {
	return strings.TrimPrefix(s, PREFIX)
}

func renewSegmentID(txID string) string {
	parts := strings.Split(txID, "|")
	if len(parts) < 3 {
		return txID
	}
	callID := parts[0]
	cseq := strings.Replace(parts[1], "_", " ", 1)
	method := utils.ExtractMethodFromCseq(cseq)
	ip := utils.GetNetworkInterfaceIP("eth0")
	return fmt.Sprintf("%s.%s.%s", callID, method, ip)
}
