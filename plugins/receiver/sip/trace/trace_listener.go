package trace

import (
	"fmt"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/sniffdata"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/utils"
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
	// 由于 UAC 和 UAS 的请求创建的 transactionid 相同，直接作为 segementID 会导致冲突
	// 因此这里对 UAS 的请求做特殊处理，当 UAS 收到请求时，使用 Call-id.Method.LocalIP 作为 segmentID
	// UAC 则继续使用 Call-id.Method.via[0].branch作为 segmentID
	// 所以 segmentObjet 使用的 tranceID 直接用 Call-id 即可
	ctx, exist := l.manager.GetTraceContextByTransactionID(req.CallID())
	if !exist {
		ctx = l.manager.CreateTraceContext(req.CallID(), req.CreatedAt())
		ctx.initSegmentObject()
	}
	switch ua {
	case types.UAClient:
		ctx.segment.TraceSegmentId = fmt.Sprintf("%s.%s.%s", req.CallID(), req.MethodAsString(), req.ViaBranch())
	case types.UAServer:
		ctx.segment.TraceSegmentId = fmt.Sprintf("%s.%s.%s", req.CallID(), req.MethodAsString(), req.DstURI())
	}
}

func (l *TraceListener) OnTransactionCreated(transaction types.Transaction) {
	ctx, exist := l.manager.GetTraceContextByTransactionID(transaction.ID())
	if !exist {
		log.Logger.Errorf("trace context not found for tx: %s", transaction.ID())
		return
	}

	method := transaction.Request().MethodAsString()
	startTime := transaction.CreatedAt()
	headers := transaction.Request().Headers()
	switch transaction.UA() {
	case types.UAClient:
		remoteURI, _ := utils.ExtractURIAndTag(transaction.Request().To())
		ctx.CreateNewSpan(transaction.ID(), method, remoteURI, startTime, headers, len(transaction.Request().Via()) == 1, nil)
	case types.UAServer:
		// 对于服务器端请求，使用From作为对端地址
		remoteURI, _ := utils.ExtractURIAndTag(transaction.Request().From())
		ref := sniffdata.NewSegmentReferenceBuilder().
			WithNetworkAddressUsedAtPeer(remoteURI).
			WithParentEndpoint(method).
			WithParentTraceSegmentID(fmt.Sprintf("%s.%s.%s", transaction.Request().CallID(), transaction.Request().MethodAsString(), transaction.Request().ViaBranch())).
			WithTraceID(fmt.Sprintf("SNIFFER-%s", transaction.Request().CallID())).
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
