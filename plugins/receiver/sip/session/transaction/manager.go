package transaction

import (
	"sync"
	"time"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/utils"
)

type TransactionManager struct {
	store     *sync.Map // 使用事务ID作为标识
	listeners []types.TransactionListener
}

func NewTransactionManager() *TransactionManager {
	m := &TransactionManager{
		store:     &sync.Map{},
		listeners: make([]types.TransactionListener, 0),
	}
	go func() {
		for {
			m.RemoveTerminatedTransaction()
			m.printMetrics()
			time.Sleep(1 * time.Minute)
		}
	}()
	return m
}

func (m *TransactionManager) RemoveTerminatedTransaction() {
	todo := make([]string, 0)
	m.store.Range(func(key, value interface{}) bool {
		tx := value.(*TransactionContext)
		if tx.state.IsTerminated() && time.Since(time.UnixMilli(tx.UpdatedAt())) > 2*time.Minute {
			todo = append(todo, key.(string))
		} else if time.Since(time.UnixMilli(tx.UpdatedAt())) > 5*time.Minute {
			// 超过5分钟的事务，强制删除
			log.Logger.WithField("Transaction-ID", tx.ID()).Warnf("Force removing long-lived transaction, last updated at %d", tx.UpdatedAt())
			todo = append(todo, key.(string))
		}
		return true
	})
	for _, id := range todo {
		m.store.Delete(id)
	}
	log.Logger.Infof("Removed terminated transaction size: %d", len(todo))
}

func (m *TransactionManager) printMetrics() {
	length := 0
	m.store.Range(func(_, _ interface{}) bool {
		length++
		return true
	})
	log.Logger.Infof("current trace context map size: %d", length)
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
