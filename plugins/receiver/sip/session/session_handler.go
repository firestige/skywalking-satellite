package session

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
	listeners     []types.SessionListener
}

func NewSessionHandler() *SessionHandler {
	return &SessionHandler{
		dialogManager: dialog.NewDialogManager(),
		txManager:     transaction.NewTransactionManager(),
	}
}

func (sh *SessionHandler) RegisterListener(listener interface{}) {
	if sessionListener, ok := listener.(types.SessionListener); ok {
		sh.listeners = append(sh.listeners, sessionListener)
		log.Logger.Debugf("Registered session listener: %T", listener)
	}
	if dialogListener, ok := listener.(types.DialogListener); ok {
		sh.dialogManager.RegisterListener(dialogListener)
		log.Logger.Debugf("Registered dialog listener: %T", listener)
	}
	if txListener, ok := listener.(types.TransactionListener); ok {
		sh.txManager.RegisterListener(txListener)
		log.Logger.Debugf("Registered transaction listener: %T", listener)
	}
}

func (sh *SessionHandler) HandleMessage(msg types.SipMessage) {
	log.Logger.WithField("Call-id", msg.CallID()).WithField("CSeq", msg.CSeq()).Debugf("Handling SIP message: %s", msg.StartLine())
	// 1. Session Listener 处理
	if msg.IsRequest() {
		for _, listener := range sh.listeners {
			listener.OnRequest(msg.(types.SipRequest), utils.ParseUAType(msg))
		}
	}

	var err error
	// dialog必须分开处理的原因是terminated事件的处理顺序影响了上报
	// 2. Dialog 处理请求
	if req, ok := msg.(types.SipRequest); ok {
		err := sh.dialogManager.HandleMessage(req)
		if err != nil {
			log.Logger.Warnf("Error handling request in dialog: %v", err)
		}
	}

	// 3. Transaction 处理
	err = sh.txManager.HandleMessage(msg)
	if err != nil {
		log.Logger.Errorf("Error handling message in transaction: %v", err)
	}

	// 4. Dialog 处理响应
	if resp, ok := msg.(types.SipResponse); ok {
		err = sh.dialogManager.HandleMessage(resp)
		if err != nil {
			log.Logger.Warnf("Error handling request in dialog: %v", err)
		}
	}
	log.Logger.WithField("Call-id", msg.CallID()).WithField("CSeq", msg.CSeq()).Debug("Finished handling SIP message")
}
