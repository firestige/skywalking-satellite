package sip

import (
	"strconv"
	"sync"
	"time"

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

	// 上下文计数器配置
	CounterCleanupInterval = 1 * time.Minute // 清理检查间隔
	CounterTTL             = 5 * time.Minute // 计数器过期时间
)

// CounterContext 表示单个 traceID 的上下文计数器
type CounterContext struct {
	count      int64     // 计数器值
	lastAccess time.Time // 最后访问时间
}

// ContextCounterManager 管理所有 traceID 的上下文计数器
type ContextCounterManager struct {
	counters map[string]*CounterContext
	mutex    sync.RWMutex
	stopCh   chan struct{}
}

// NewContextCounterManager 创建新的上下文计数器管理器
func NewContextCounterManager() *ContextCounterManager {
	manager := &ContextCounterManager{
		counters: make(map[string]*CounterContext),
		stopCh:   make(chan struct{}),
	}

	// 启动清理 goroutine
	go manager.cleanupExpiredCounters()

	return manager
}

// GetAndIncrement 获取并递增指定 segmentID 的计数器
func (m *ContextCounterManager) GetAndIncrement(segmentID string) int64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()

	// 检查是否存在计数器
	if counter, exists := m.counters[segmentID]; exists {
		counter.count++
		counter.lastAccess = now
		return counter.count
	}

	// 创建新的计数器，从 0 开始，第一次调用返回 0
	m.counters[segmentID] = &CounterContext{
		count:      0,
		lastAccess: now,
	}

	return 0
}

// GetCurrentCount 获取指定 segmentID 的当前计数（不递增）
func (m *ContextCounterManager) GetCurrentCount(segmentID string) int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if counter, exists := m.counters[segmentID]; exists {
		return counter.count
	}
	return 0
}

// cleanupExpiredCounters 清理过期的计数器
func (m *ContextCounterManager) cleanupExpiredCounters() {
	ticker := time.NewTicker(CounterCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.performCleanup()
		case <-m.stopCh:
			return
		}
	}
}

// performCleanup 执行清理操作
func (m *ContextCounterManager) performCleanup() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	// 收集过期的 keys
	for segmentID, counter := range m.counters {
		if now.Sub(counter.lastAccess) > CounterTTL {
			expiredKeys = append(expiredKeys, segmentID)
		}
	}

	// 删除过期的计数器
	for _, key := range expiredKeys {
		delete(m.counters, key)
		log.Logger.Debugf("Cleaned up expired counter for segmentID: %s", key)
	}

	if len(expiredKeys) > 0 {
		log.Logger.Infof("Cleaned up %d expired counters", len(expiredKeys))
	}
}

// Stop 停止计数器管理器
func (m *ContextCounterManager) Stop() {
	close(m.stopCh)
}

// GetStats 获取统计信息（用于监控）
func (m *ContextCounterManager) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return map[string]interface{}{
		"active_counters": len(m.counters),
		"timestamp":       time.Now().Unix(),
	}
}

type Receiver struct {
	config.CommonFields

	OutputChannel  chan *v1.SniffData
	Server         *packet.Server
	counterManager *ContextCounterManager
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
	r.counterManager = NewContextCounterManager()
	r.Server.RegisterHandler("UDP", "sip", r.packetHandler)
}

func (r *Receiver) RegisterSyncInvoker(_ module.SyncInvoker) {
	// No sync invoker needed for SIP receiver
}

func (r *Receiver) packetHandler(data *types.RawFrameData) error {
	segment := r.buildSegment(data)
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
	packet := buildLogData(data, segment.TraceId, segment.TraceSegmentId, segment.Spans[0].SpanId)
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

func (r *Receiver) buildSegment(source *types.RawFrameData) *agent.SegmentObject {
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

	// 使用上下文计数器管理器获取并递增计数器（基于 segmentId）
	segmentIdStr := segmentId.Value()
	contextCounter := r.counterManager.GetAndIncrement(segmentIdStr)

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
		TraceSegmentId:  segmentIdStr,
		TraceId:         traceId.Value(),
		Service:         "SIP Service",
		ServiceInstance: "SIP Instance",
		Spans: []*agent.SpanObject{
			{
				SpanId:        int32(contextCounter), // 使用上下文计数器作为 SpanId，从 0 开始
				ParentSpanId:  -1,
				StartTime:     source.Timestamp,
				EndTime:       source.Timestamp + 1000, // 假设处理时间为1000毫秒
				OperationName: operationName,
				SpanType:      agent.SpanType_Local,
				SpanLayer:     agent.SpanLayer_Unknown,
				IsError:       isError,
			},
		},
	}
}

func buildLogData(source *types.RawFrameData, traceId string, segmentId string, spanId int32) *logging.LogData {
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
			TraceId:        traceId,
			TraceSegmentId: segmentId,
			SpanId:         spanId,
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
	if r.counterManager != nil {
		r.counterManager.Stop()
	}
}
