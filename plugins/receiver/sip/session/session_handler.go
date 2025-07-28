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
	dx, exist := sh.dialogManager.GetDialogBySipMessage(msg)
	if !exist {
		dx = sh.dialogManager.CreateDialog(msg.(types.SipRequest))
		if dx != nil {
			log.Logger.Debugf("Created new dialog: %s", dx.ID())
			dx.HandleMessage(msg)
		}
	}

	tx, exist := sh.txManager.GetTransactionBySipMessage(msg)
	if !exist {
		tx = sh.txManager.CreateTransaction(msg.(types.SipRequest))
		if tx != nil {
			log.Logger.Debugf("Created new transaction: %s", tx.ID())
			tx.HandleMessage(msg)
		}
	}
}
