package transaction

import (
	"time"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/utils"
)

type PendingResponseContainer struct {
	responses    []types.SipResponse
	lastUpdateAt time.Time
}

type TransactionManager struct {
	store     map[string]*TransactionContext // 使用事务ID作为标识
	listeners []types.TransactionListener
	buffer    map[string]*PendingResponseContainer
}

func NewTransactionManager() *TransactionManager {
	return &TransactionManager{
		store:     make(map[string]*TransactionContext),
		listeners: make([]types.TransactionListener, 0),
		buffer:    make(map[string]*PendingResponseContainer),
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
		m.store[tx.ID()] = tx
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
		} else {
			// 有可能请求还没到，先缓存响应
			txID := utils.BuildTransactionID(msg)
			resp := msg.(types.SipResponse)
			container, exists := m.buffer[txID]
			if !exists {
				container = &PendingResponseContainer{
					responses:    make([]types.SipResponse, 0),
					lastUpdateAt: time.Now(),
				}
				m.buffer[txID] = container
			}
			container.responses = append(container.responses, resp)
			container.lastUpdateAt = time.Now()
			return nil // 直接返回，等待下次请求到达
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
				listener.OnTransactionTerminated(tx)
			}
		}
	}
	// 处理完请求查看是否有没处理的响应
	txID := tx.ID()
	container, exists := m.buffer[txID]
	if exists {
		for _, resp := range container.responses {
			tx.HandleMessage(resp)
		}
		delete(m.buffer, txID)
	}
	return err
}

func (m *TransactionManager) GetTransactionByID(id string) (*TransactionContext, bool) {
	ctx, exists := m.store[id]
	return ctx, exists
}

func (m *TransactionManager) GetTransactionBySipMessage(msg types.SipMessage) (*TransactionContext, bool) {
	txID := utils.BuildTransactionID(msg)
	ctx, exists := m.store[txID]
	return ctx, exists
}

func (m *TransactionManager) GetAllTransactions() []*TransactionContext {
	var allTransactions []*TransactionContext
	for _, ctx := range m.store {
		allTransactions = append(allTransactions, ctx)
	}
	return allTransactions
}

func (m *TransactionManager) Clear() {
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			m.clearExpiredPairs()
		}
	}()
}

func (m *TransactionManager) clearExpiredPairs() {
	now := time.Now()
	expiredThreshold := 30 * time.Second

	for txID, pair := range m.buffer {
		if now.Sub(pair.lastUpdateAt) > expiredThreshold {
			delete(m.buffer, txID)
			log.Logger.WithField("Transaction-ID", txID).
				Infof("Cleared expired transaction pair, last updated: %s", pair.lastUpdateAt.Format(time.RFC3339))
		}
	}
}
