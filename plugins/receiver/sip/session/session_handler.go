package session

import (
	"github.com/apache/skywalking-satellite/internal/pkg/log"

	"github.com/apache/skywalking-satellite/plugins/receiver/sip/session/transaction"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/utils"
)

type SessionHandler struct {
	txManager *transaction.TransactionManager
	listeners []types.SessionListener
}

func NewSessionHandler() *SessionHandler {
	return &SessionHandler{
		txManager: transaction.NewTransactionManager(),
	}
}

func (sh *SessionHandler) RegisterListener(listener interface{}) {
	if sessionListener, ok := listener.(types.SessionListener); ok {
		sh.listeners = append(sh.listeners, sessionListener)
		log.Logger.Debugf("Registered session listener: %T", listener)
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

	// 2. Transaction 处理
	err := sh.txManager.HandleMessage(msg)
	if err != nil {
		log.Logger.Errorf("Error handling message in transaction: %v", err)
	}
	log.Logger.WithField("Call-id", msg.CallID()).WithField("CSeq", msg.CSeq()).Debug("Finished handling SIP message")
}
