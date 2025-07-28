package session

import (
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/session/dialog"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/session/transaction"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

type SessionHandler struct {
	dialogManager *dialog.DialogManager
	txManager     *transaction.TransactionManager
}

func NewSessionHandler() *SessionHandler {
	return &SessionHandler{
		dialogManager: dialog.NewDialogManager(),
		txManager:     transaction.NewTransactionManager(),
	}
}

func (sh *SessionHandler) HandleMessage(msg types.SipMessage) {
	// 1. Dialog 处理
	dx, exist := sh.dialogManager.GetDialogBySipMessage(msg)
	if !exist && msg.IsRequest() {
		if req, ok := msg.(types.SipRequest); ok {
			dx = sh.dialogManager.CreateDialog(req)
			if dx != nil {
				log.Logger.Debugf("Created new dialog: %s", dx.ID())
			}
		}
	}
	if dx != nil {
		dx.HandleMessage(msg)
	}

	// 2. Transaction 处理
	tx, exist := sh.txManager.GetTransactionBySipMessage(msg)
	if !exist && msg.IsRequest() {
		if req, ok := msg.(types.SipRequest); ok {
			tx = sh.txManager.CreateTransaction(req)
			if tx != nil {
				log.Logger.Debugf("Created new transaction: %s", tx.ID())
			}
		}
	}
	if tx != nil {
		tx.HandleMessage(msg)
	}
}
