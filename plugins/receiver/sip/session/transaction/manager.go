package transaction

import (
	"sync"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/utils"
)

type TransactionManager struct {
	store     *sync.Map // 使用事务ID作为标识
	listeners []types.TransactionListener
}

func NewTransactionManager() *TransactionManager {
	return &TransactionManager{
		store:     &sync.Map{},
		listeners: make([]types.TransactionListener, 0),
	}
}

func (m *TransactionManager) RegisterListener(listener types.TransactionListener) {
	m.listeners = append(m.listeners, listener)
}

func (m *TransactionManager) CreateTransaction(msg types.SipMessage) *TransactionContext {
	if req, ok := msg.(types.SipRequest); ok {
		ua := utils.ParseUAType(req)
		var state TransactionState
		switch req.Method() {
		case types.MethodInvite:
			// 对于Invite请求，客户端和服务器端的处理逻辑不同
			if ua == types.UAClient {
				state = &InviteCallingState{}
			}
			if ua == types.UAServer {
				state = &InviteProceedingState{}
			}
		default:
			state = &NonInviteTryingState{}
		}
		if state == nil {
			log.Logger.WithField("Call-id", req.CallID()).WithField("ua", ua).Errorf("Unsupported request msg: %s", req.StartLine())
			return nil
		}
		tx := NewTransaction(req, state)
		m.store.Store(tx.ID(), tx)
		for _, listener := range m.listeners {
			listener.OnTransactionCreated(tx)
		}
		return tx
	}
	return nil
}

func (m *TransactionManager) HandleMessage(msg types.SipMessage) error {
	tx, exist := m.GetTransactionBySipMessage(msg)
	if !exist {
		if msg.IsRequest() {
			if req, ok := msg.(types.SipRequest); ok {
				tx = m.CreateTransaction(req)
				if tx != nil {
					log.Logger.Infof("Created new transaction: %s", tx.ID())
				}
			}
		}
	}
	if tx == nil {
		log.Logger.Warnf("No transaction found for message: %s", msg.StartLine())
		return nil
	}
	err := tx.HandleMessage(msg)
	if err == nil {
		if tx.state.IsTerminated() {
			for _, listener := range m.listeners {
				log.Logger.WithField("Transaction-ID", tx.ID()).Infof("Transaction terminated at %d", tx.UpdatedAt())
				listener.OnTransactionTerminated(tx)
			}
		}
	}
	return err
}

func (m *TransactionManager) GetTransactionByID(id string) (*TransactionContext, bool) {
	ctx, exists := m.store.Load(id)
	if ctx == nil {
		return nil, exists
	} else {
		return ctx.(*TransactionContext), exists
	}
}

func (m *TransactionManager) GetTransactionBySipMessage(msg types.SipMessage) (*TransactionContext, bool) {
	txID := utils.BuildTransactionID(msg)
	ctx, exists := m.store.Load(txID)
	if ctx == nil {
		return nil, exists
	} else {
		return ctx.(*TransactionContext), exists
	}
}

func (m *TransactionManager) GetAllTransactions() []*TransactionContext {
	var allTransactions []*TransactionContext
	m.store.Range(func(key, value interface{}) bool {
		allTransactions = append(allTransactions, value.(*TransactionContext))
		return true
	})
	return allTransactions
}
