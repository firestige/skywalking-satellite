package trace

import (
	"fmt"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/sniffdata"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/utils"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"
)

type TraceListener struct {
	serviceName       string
	serviceInstanceId string
	manager           *TraceManager
	submit            func(*v1.SniffData)
}

func NewTraceListener(serviceName, serviceInstanceId string, submit func(*v1.SniffData)) *TraceListener {
	return &TraceListener{
		serviceName:       serviceName,
		serviceInstanceId: serviceInstanceId,
		manager:           NewTraceManager(serviceName, serviceInstanceId),
		submit:            submit,
	}
}

func (l *TraceListener) OnRequest(req types.SipRequest, ua types.UAType) {

}

func (l *TraceListener) OnTransactionCreated(transaction types.Transaction) {
	ctx := l.manager.CreateTraceContext(transaction.ID(), transaction.CreatedAt())
	ctx.initSegmentObject()

	req := transaction.Request()
	ctx.segment.TraceSegmentId = fmt.Sprintf("%s.%s.%s.%d", req.CallID(), req.CSeq(), req.ViaBranch(), transaction.UA())

	method := transaction.Request().MethodAsString()
	startTime := transaction.CreatedAt()
	headers := transaction.Request().Headers()
	switch transaction.UA() {
	case types.UAClient:
		remoteURI, _ := utils.ExtractURIAndTag(transaction.Request().To())
		var ref *agent.SegmentReference
		ref = nil
		if len(req.Via()) > 1 {
			ref = sniffdata.NewSegmentReferenceBuilder().
				WithNetworkAddressUsedAtPeer(remoteURI).
				WithTraceID(ctx.traceID).
				WithParentTraceSegmentID(fmt.Sprintf("%s.%s.%s.%d", req.CallID(), req.CSeq(), utils.GetBranchFromVia(req.Via()[1]), types.UAServer)).
				Build()
			parentTxID := fmt.Sprintf("%s.%s.%s", req.CallID(), req.CSeq(), utils.GetBranchFromVia(req.Via()[1]))
			pctx, exist := l.manager.GetTraceContextByTransactionID(parentTxID)
			if exist {
				// 如果找到了父事务的TraceContext，说明这是一个嵌套的调用，更新状态
				pctx.isProxyNode = true
			}
		}
		ctx.CreateNewSpan(transaction.ID(), method, remoteURI, startTime, headers, ref == nil, ref)
	case types.UAServer:
		// 对于服务器端请求，使用From作为对端地址
		remoteURI, _ := utils.ExtractURIAndTag(transaction.Request().From())
		ref := sniffdata.NewSegmentReferenceBuilder().
			WithNetworkAddressUsedAtPeer(remoteURI).
			WithTraceID(ctx.traceID).
			WithParentSpanID(0).
			WithParentTraceSegmentID(fmt.Sprintf("%s.%s.%s.%d", req.CallID(), req.CSeq(), req.ViaBranch(), types.UAClient)).
			Build()
		ctx.CreateNewSpan(transaction.ID(), method, remoteURI, startTime, headers, false, ref)
	}
}

func (l *TraceListener) OnTransactionStateChanged(transaction types.Transaction) {
	// 一般transaction状态变化时不需要更新Segment和Span
}

func (l *TraceListener) OnTransactionTerminated(tx types.Transaction) {
	log.Logger.Debugf("Transaction terminated: %s", tx.ID())
	ctx, exist := l.manager.GetTraceContextByTransactionID(tx.ID())
	if !exist {
		log.Logger.Errorf("trace context not found for tx: %s", tx.ID())
		return
	}
	isError := tx.LastResponse() != nil && tx.LastResponse().Status() >= 300
	msgs := make([]types.SipMessage, 0, 1+len(tx.Responses()))
	msgs = append(msgs, tx.Request())
	for _, resp := range tx.Responses() {
		msgs = append(msgs, resp)
	}
	ctx.addMsgToSpan(msgs)
	// 结束现有的Span
	ctx.FinishExistSpan(tx.ID(), isError, tx.UpdatedAt())

	// 如果是最后一个节点，刷新 SegmentId 以匹配 kafka事件中 parent Segment ID 的生成逻辑
	if !ctx.isProxyNode {
		ctx.segment.TraceSegmentId = renewSegmentID(tx.ID())
	}

	data := sniffdata.WrapWithSniffData(ctx.segment) // 发送Segment
	l.submit(data)                                   // 提交Segment到输出通道
	l.manager.RemoveTraceContextByTransactionID(tx.Request().CallID())
}

func (l *TraceListener) OnTransactionTimeout(tx types.Transaction) {
	ctx, exist := l.manager.GetTraceContextByTransactionID(tx.Request().CallID())
	if !exist {
		log.Logger.Errorf("trace context not found for dialog with Call-ID: %s", tx.Request().CallID())
		return
	}
	ctx.FinishExistSpan(tx.ID(), true, tx.UpdatedAt()) // 超时场景一定是错误
	data := sniffdata.WrapWithSniffData(ctx.segment)   // 发送Segment
	l.submit(data)                                     // 提交Segment到输出通道
	l.manager.RemoveTraceContextByTransactionID(tx.Request().CallID())
}

func (l *TraceListener) OnTransactionError(tx types.Transaction, err error) {
	ctx, exist := l.manager.GetTraceContextByTransactionID(tx.Request().CallID())
	if !exist {
		log.Logger.Errorf("trace context not found for dialog with Call-ID: %s", tx.Request().CallID())
		return
	}
	ctx.FinishExistSpan(tx.ID(), true, tx.UpdatedAt()) // 错误场景一定是错误
	data := sniffdata.WrapWithSniffData(ctx.segment)   // 发送Segment
	l.submit(data)                                     // 提交Segment到输出通道
	l.manager.RemoveTraceContextByTransactionID(tx.Request().CallID())
}
