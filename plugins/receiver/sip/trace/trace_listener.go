package sip

import (
	"fmt"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/utils"
)

type TraceListener struct {
	serviceName       string
	serviceInstanceId string
	manager           *TraceManager
}

func NewTraceListener(serviceName, serviceInstanceId string) *TraceListener {
	return &TraceListener{
		serviceName:       serviceName,
		serviceInstanceId: serviceInstanceId,
		manager:           NewTraceManager(serviceName, serviceInstanceId),
	}
}

func (l *TraceListener) OnRequestSent(req types.SipRequest) {
	l.initContext(req, types.UAClient)
}

func (l *TraceListener) OnRequestReceived(req types.SipRequest) {
	l.initContext(req, types.UAServer)
}

func (l *TraceListener) initContext(req types.SipRequest, uaType types.UAType) {
	switch req.Method() {
	// TODO 只处理特定的SIP方法，后续应该做成可配置的
	case types.MethodInvite, types.MethodRegister, types.MethodOptions:
		// 首先根据提取TraceID的策略处理请求
		traceID := GetTraceIDFromRequest(req)
		ctx, exists := l.manager.GetTraceContextByTraceID(traceID)
		if exists {
			// 首个请求不应该有TraceContext，需要处理异常
			err := fmt.Errorf("trace context already exists for trace ID %s", traceID)
			log.Logger.WithError(err).Errorf("failed to inital segment: %s", traceID)
			return
		}
		ctx = l.manager.CreateTraceContext(traceID, req.CreatedAt())
		l.manager.AliasWithCallID(traceID, req.CallID())
		ctx.initSegmentObject(req, uaType)
	}
}

// TODO 根据实际情况修改
func GetTraceIDFromRequest(req types.SipRequest) string {
	panic("unimplemented")
}

func (l *TraceListener) OnDialogCreated(dialog types.Dialog) {
	ctx, exist := l.manager.GetTraceContextByCallID(dialog.CallID())
	if !exist {
		log.Logger.Errorf("trace context not found for dialog with Call-ID: %s", dialog.CallID())
		return
	}
	ctx.CreateNewSpan(dialog.ID(), dialog.CallID(), string(types.MethodInvite), dialog.RemoteURI(), dialog.CreatedAt(), dialog.Metadatas())
}

func (l *TraceListener) OnDialogStateChanged(dialog types.Dialog) {
	// 一般dialog状态变化时不需要更新Segment和Span
}

func (l *TraceListener) OnDialogTerminated(dialog types.Dialog) {
	ctx, exist := l.manager.GetTraceContextByCallID(dialog.CallID())
	if !exist {
		log.Logger.Errorf("trace context not found for dialog with Call-ID: %s", dialog.CallID())
		return
	}
	ctx.FinishExistSpan(dialog.ID(), false, dialog.UpdatedAt()) // 我们认为事务有成功与失败，会话没有
}

func (l *TraceListener) OnTransactionCreated(transaction types.Transaction) {
	ctx, exist := l.manager.GetTraceContextByCallID(transaction.Request().CallID())
	if !exist {
		log.Logger.Errorf("trace context not found for dialog with Call-ID: %s", transaction.Request().CallID())
		return
	}
	// 创建新的Span
	callID := transaction.Request().CallID()
	method := transaction.Request().MethodAsString()
	startTime := transaction.CreatedAt()
	headers := transaction.Request().Headers()
	switch transaction.UA() {
	case types.UAClient:
		remoteURI, _ := utils.ExtractURIAndTag(transaction.Request().To())
		ctx.CreateNewSpan(transaction.ID(), callID, method, remoteURI, startTime, headers)
	case types.UAServer:
		// 对于服务器端请求，使用From作为对端地址
		remoteURI, _ := utils.ExtractURIAndTag(transaction.Request().From())
		ctx.CreateNewSpan(transaction.ID(), callID, method, remoteURI, startTime, headers)
	}
}

func (l *TraceListener) OnTransactionStateChanged(transaction types.Transaction) {
	// 一般dialog状态变化时不需要更新Segment和Span
}

func (l *TraceListener) OnTransactionTerminated(transaction types.Transaction) {
	ctx, exist := l.manager.GetTraceContextByCallID(transaction.Request().CallID())
	if !exist {
		log.Logger.Errorf("trace context not found for dialog with Call-ID: %s", transaction.Request().CallID())
		return
	}
	isError := transaction.LastResponse() != nil && transaction.LastResponse().Status() >= 300
	// 结束现有的Span
	ctx.FinishExistSpan(transaction.ID(), isError, transaction.UpdatedAt())
}

func (l *TraceListener) OnTransactionTimeout(transaction types.Transaction) {
	ctx, exist := l.manager.GetTraceContextByCallID(transaction.Request().CallID())
	if !exist {
		log.Logger.Errorf("trace context not found for dialog with Call-ID: %s", transaction.Request().CallID())
		return
	}
	ctx.FinishExistSpan(transaction.ID(), true, transaction.UpdatedAt()) // 超时场景一定是错误
}

func (l *TraceListener) OnTransactionError(transaction types.Transaction, err error) {
	ctx, exist := l.manager.GetTraceContextByCallID(transaction.Request().CallID())
	if !exist {
		log.Logger.Errorf("trace context not found for dialog with Call-ID: %s", transaction.Request().CallID())
		return
	}
	ctx.FinishExistSpan(transaction.ID(), true, transaction.UpdatedAt()) // 会话错误场景一定是错误
}
