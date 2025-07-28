package sip

import (
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/session/dialog"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/session/transaction"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/utils"
)

type SessionHandler struct {
	dialogManager *dialog.DialogManager
	txManager     *transaction.TransactionManager
}

func (sh *SessionHandler) HandleMessage(msg types.SipMessage) {
	dctx, exist := sh.dialogManager.GetDialogBySipMessage(msg)
	if !exist {
		dctx, err := sh.dialogManager.CreateDialog(msg.(types.SipRequest))
		if err != nil {
			log.Logger.WithError(err).Errorf("Failed to create dialog for message: %s", msg.String())
			return
		}
		log.Logger.Debugf("Created new dialog: %s", dctx.ID())
	}

	ua := utils.ParseUAType(msg)
	switch ua {
	case types.UAClient:
		sh.handleClientMessage(msg)
	case types.UAServer:
		sh.handleServerMessage(msg)
	default:
		// 未知UA类型，忽略
		return
	}
}

func (sh *SessionHandler) handleServerMessage(msg types.SipMessage) {
	if req, ok := msg.(types.SipRequest); ok {
		sh.onUASreceiveRequest(req)
	}
	if resp, ok := msg.(types.SipResponse); ok {
		sh.onUASsendResponse(resp)
	}
	log.Logger.WithError(types.ErrNotFound).Debugf("Received SIP message: %s", msg.String())
}

func (sh *SessionHandler) handleClientMessage(msg types.SipMessage) {
	if req, ok := msg.(types.SipRequest); ok {
		sh.onUACsendRequest(req)
	}
	if resp, ok := msg.(types.SipResponse); ok {
		sh.onUACreceiveResponse(resp)
	}
	log.Logger.WithError(types.ErrNotFound).Debugf("Received SIP message: %s", msg.String())
}

func (sh *SessionHandler) onUACsendRequest(req types.SipRequest) {
	// 处理发送请求的逻辑
	log.Logger.Debugf("Sending SIP request: %s", req.String())
	event := &types.SipEvent{
		Type:      types.EventSendRequest,
		Name:      "EventSendRequest",
		Message:   req,
		Context:   make(map[string]interface{}),
		Timestamp: req.CreatedAt(),
	}
	switch req.Method() {
	case types.MethodAck:

	case types.MethodInvite:
		// 处理邀请、注册和订阅请求
		ctx := sh.txManager.CreateTransaction(req)

		ctx.HandleMessage(event)
	default:
		ctx, exist := sh.txManager.GetTransactionBySipMessage(req)
		if !exist {
			log.Logger.WithError(types.ErrNotFound).Debug("ignore request: %s", req.String())
		}
		ctx.HandleEvent(event)
	}
}

func (sh *SessionHandler) onUACreceiveResponse(resp types.SipResponse) {
	// 处理接收响应的逻辑
	log.Logger.Debugf("Received SIP response: %s", resp.String())
}

func (sh *SessionHandler) onUASreceiveRequest(req types.SipRequest) {
	// 处理接收请求的逻辑
	log.Logger.Debugf("Received SIP request: %s", req.String())
}

func (sh *SessionHandler) onUASsendResponse(resp types.SipResponse) {
	// 处理发送响应的逻辑
	log.Logger.Debugf("Sending SIP response: %s", resp.String())
}
