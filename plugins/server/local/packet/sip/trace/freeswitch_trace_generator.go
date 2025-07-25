package trace

import (
	"fmt"
	"sync"
	"time"

	"github.com/apache/skywalking-satellite/plugins/server/local/packet/sip/events"
	v3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

type FreeSwitchTraceGenerator struct {
	// 追踪上下文管理
	eslSessions  map[string]*ESLTraceContext // ESL会话ID -> 追踪上下文
	dialogs      map[string]*DialogContext   // Dialog ID -> Dialog上下文
	transactions map[string]*TransactionSpan // Transaction ID -> 事务span

	segmentBuilder *SegmentBuilder
	mutex          sync.RWMutex
}

// ESL追踪上下文（根span）
type ESLTraceContext struct {
	TraceID    string
	SegmentID  string
	RootSpanID int32
	RootSpan   *v3.SpanObject

	// 关联的dialogs
	DialogSpans map[string]int32 // Dialog ID -> Dialog Span ID

	StartTime  time.Time
	ESLCommand string // originate, bridge等
	CallID     string // FreeSwitch的Call ID
}

// Dialog上下文（兄弟span）
type DialogContext struct {
	DialogID     string
	SpanID       int32
	ParentSpanID int32 // Root Span ID
	DialogSpan   *v3.SpanObject

	// Dialog内的事务spans
	Transactions map[string]int32 // CSeq -> Transaction Span ID

	CallDirection string // "outbound_a" / "outbound_b"
	RemoteURI     string
	State         string
}

// 事务span（请求-响应对）
type TransactionSpan struct {
	SpanID       int32
	ParentSpanID int32 // Dialog Span ID
	Span         *v3.SpanObject

	Method       string
	CSeq         string
	RequestTime  time.Time
	ResponseTime time.Time
	StatusCode   int
	IsCompleted  bool
}

func NewFreeSwitchTraceGenerator() *FreeSwitchTraceGenerator {
	return &FreeSwitchTraceGenerator{
		eslSessions:    make(map[string]*ESLTraceContext),
		dialogs:        make(map[string]*DialogContext),
		transactions:   make(map[string]*TransactionSpan),
		segmentBuilder: NewSegmentBuilder(),
	}
}

// 处理ESL事件（创建root span）
func (ftg *FreeSwitchTraceGenerator) HandleESLEvent(eslSessionID, command, callID string) error {
	ftg.mutex.Lock()
	defer ftg.mutex.Unlock()

	traceID := generateTraceID()
	segmentID := generateSegmentID()
	rootSpanID := int32(0)

	rootSpan := &v3.SpanObject{
		SpanId:        rootSpanID,
		ParentSpanId:  -1, // 根span
		StartTime:     time.Now().UnixNano() / 1e6,
		OperationName: fmt.Sprintf("FreSwitch_%s", command),
		SpanType:      v3.SpanType_Entry,
		SpanLayer:     v3.SpanLayer_RPC_FRAMEWORK,
		ComponentId:   getSIPComponentID(),
		Tags: []*v3.KeyValue{
			{Key: "esl.command", Value: command},
			{Key: "esl.session_id", Value: eslSessionID},
			{Key: "fs.call_id", Value: callID},
			{Key: "component", Value: "freeswitch"},
		},
	}

	eslContext := &ESLTraceContext{
		TraceID:     traceID,
		SegmentID:   segmentID,
		RootSpanID:  rootSpanID,
		RootSpan:    rootSpan,
		DialogSpans: make(map[string]int32),
		StartTime:   time.Now(),
		ESLCommand:  command,
		CallID:      callID,
	}

	ftg.eslSessions[eslSessionID] = eslContext

	return ftg.segmentBuilder.AddSpan(segmentID, rootSpan)
}

// 处理SIP Dialog事件（创建兄弟span）
func (ftg *FreeSwitchTraceGenerator) HandleDialogStart(eslSessionID, dialogID, direction, remoteURI string) error {
	ftg.mutex.Lock()
	defer ftg.mutex.Unlock()

	eslContext := ftg.eslSessions[eslSessionID]
	if eslContext == nil {
		return fmt.Errorf("ESL session %s not found", eslSessionID)
	}

	// 分配dialog span ID (1, 2, 3...)
	dialogSpanID := int32(len(eslContext.DialogSpans) + 1)

	dialogSpan := &v3.SpanObject{
		SpanId:        dialogSpanID,
		ParentSpanId:  eslContext.RootSpanID, // 父span是root span
		StartTime:     time.Now().UnixNano() / 1e6,
		OperationName: fmt.Sprintf("SIP_Dialog_%s", direction),
		SpanType:      v3.SpanType_Exit, // 对外呼叫
		SpanLayer:     v3.SpanLayer_RPC_FRAMEWORK,
		ComponentId:   getSIPComponentID(),
		Tags: []*v3.KeyValue{
			{Key: "sip.dialog_id", Value: dialogID},
			{Key: "sip.direction", Value: direction},
			{Key: "sip.remote_uri", Value: remoteURI},
			{Key: "fs.call_id", Value: eslContext.CallID},
		},
	}

	dialogContext := &DialogContext{
		DialogID:      dialogID,
		SpanID:        dialogSpanID,
		ParentSpanID:  eslContext.RootSpanID,
		DialogSpan:    dialogSpan,
		Transactions:  make(map[string]int32),
		CallDirection: direction,
		RemoteURI:     remoteURI,
		State:         "INIT",
	}

	eslContext.DialogSpans[dialogID] = dialogSpanID
	ftg.dialogs[dialogID] = dialogContext

	return ftg.segmentBuilder.AddSpan(eslContext.SegmentID, dialogSpan)
}

// 处理SIP事务（创建子span）
func (ftg *FreeSwitchTraceGenerator) HandleSIPTransaction(dialogID string, event *events.SIPStateChangeEvent) error {
	ftg.mutex.Lock()
	defer ftg.mutex.Unlock()

	dialogContext := ftg.dialogs[dialogID]
	if dialogContext == nil {
		return fmt.Errorf("dialog %s not found", dialogID)
	}

	// 构建事务ID（通常是方法+CSeq）
	transactionID := fmt.Sprintf("%s_%s", event.Message.Method, event.Message.CSeq)

	// 检查是否已存在该事务
	transactionSpanID, exists := dialogContext.Transactions[transactionID]

	if !exists {
		// 创建新的事务span
		transactionSpanID = ftg.generateTransactionSpanID(dialogContext.SpanID)

		transactionSpan := &v3.SpanObject{
			SpanId:        transactionSpanID,
			ParentSpanId:  dialogContext.SpanID, // 父span是dialog span
			StartTime:     time.Now().UnixNano() / 1e6,
			OperationName: fmt.Sprintf("SIP_%s_Transaction", event.Message.Method),
			SpanType:      v3.SpanType_Local,
			SpanLayer:     v3.SpanLayer_RPC_FRAMEWORK,
			ComponentId:   getSIPComponentID(),
			Tags: []*v3.KeyValue{
				{Key: "sip.method", Value: event.Message.Method},
				{Key: "sip.cseq", Value: event.Message.CSeq},
				{Key: "sip.transaction_id", Value: transactionID},
				{Key: "sip.call_id", Value: event.Message.CallID},
			},
		}

		transaction := &TransactionSpan{
			SpanID:       transactionSpanID,
			ParentSpanID: dialogContext.SpanID,
			Span:         transactionSpan,
			Method:       event.Message.Method,
			CSeq:         event.Message.CSeq,
			RequestTime:  time.Now(),
			IsCompleted:  false,
		}

		dialogContext.Transactions[transactionID] = transactionSpanID
		ftg.transactions[transactionID] = transaction

		// 找到对应的ESL上下文和segment
		eslContext := ftg.findESLContextByDialog(dialogID)
		if eslContext != nil {
			ftg.segmentBuilder.AddSpan(eslContext.SegmentID, transactionSpan)
		}
	} else {
		// 更新现有事务（通常是收到响应）
		transaction := ftg.transactions[transactionID]
		if transaction != nil && event.Message.StatusCode > 0 {
			transaction.ResponseTime = time.Now()
			transaction.StatusCode = event.Message.StatusCode

			// 更新span
			transaction.Span.EndTime = time.Now().UnixNano() / 1e6
			transaction.Span.Tags = append(transaction.Span.Tags, &v3.KeyValue{
				Key:   "sip.status_code",
				Value: fmt.Sprintf("%d", event.Message.StatusCode),
			})

			if event.Message.StatusCode >= 400 {
				transaction.Span.IsError = true
			}

			transaction.IsCompleted = true
		}
	}

	return nil
}

// 生成事务span ID（基于dialog span ID）
func (ftg *FreeSwitchTraceGenerator) generateTransactionSpanID(dialogSpanID int32) int32 {
	// 事务span ID格式: dialogSpanID * 100 + 序号
	// 例如: Dialog 1的事务spans: 101, 102, 103...
	//      Dialog 2的事务spans: 201, 202, 203...

	baseID := dialogSpanID * 100
	maxID := baseID

	// 找到该dialog下的最大事务span ID
	for _, transaction := range ftg.transactions {
		if transaction.ParentSpanID == dialogSpanID && transaction.SpanID > maxID {
			maxID = transaction.SpanID
		}
	}

	return maxID + 1
}

// 查找dialog对应的ESL上下文
func (ftg *FreeSwitchTraceGenerator) findESLContextByDialog(dialogID string) *ESLTraceContext {
	for _, eslContext := range ftg.eslSessions {
		if _, exists := eslContext.DialogSpans[dialogID]; exists {
			return eslContext
		}
	}
	return nil
}

// 完成ESL会话（结束root span）
func (ftg *FreeSwitchTraceGenerator) CompleteESLSession(eslSessionID string) error {
	ftg.mutex.Lock()
	defer ftg.mutex.Unlock()

	eslContext := ftg.eslSessions[eslSessionID]
	if eslContext == nil {
		return fmt.Errorf("ESL session %s not found", eslSessionID)
	}

	// 结束root span
	eslContext.RootSpan.EndTime = time.Now().UnixNano() / 1e6

	// 清理资源
	delete(ftg.eslSessions, eslSessionID)

	return nil
}

// 示例：FreSwitch发起双方通话
func ExampleFreeSwitchCall() {
	ftg := NewFreeSwitchTraceGenerator()

	// 1. ESL originate命令 -> Root Span (SpanID: 0)
	ftg.HandleESLEvent("esl_session_123", "originate", "call_uuid_456")

	// 2. 呼叫用户A -> Dialog Span 1 (SpanID: 1, Parent: 0)
	ftg.HandleDialogStart("esl_session_123", "dialog_A", "outbound_a", "sip:userA@domain.com")

	// 3. 呼叫用户B -> Dialog Span 2 (SpanID: 2, Parent: 0)
	ftg.HandleDialogStart("esl_session_123", "dialog_B", "outbound_b", "sip:userB@domain.com")

	// 4. Dialog A的INVITE事务 -> Transaction Span (SpanID: 101, Parent: 1)
	ftg.HandleSIPTransaction("dialog_A", inviteEvent)

	// 5. Dialog B的INVITE事务 -> Transaction Span (SpanID: 201, Parent: 2)
	ftg.HandleSIPTransaction("dialog_B", inviteEvent)

	// ... 更多SIP事务

	// 6. 完成ESL会话
	ftg.CompleteESLSession("esl_session_123")
}
