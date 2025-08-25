package transaction

import (
	"time"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/utils"
)

type TransactionPair struct {
	request       types.SipRequest
	response      types.SipResponse
	lastUopdateAt time.Time
}

type TransactionManager struct {
	store     map[string]*TransactionContext // 使用事务ID作为标识
	listeners []types.TransactionListener
	buffer    map[string]*TransactionPair
}

func NewTransactionManager() *TransactionManager {
	return &TransactionManager{
		store:     make(map[string]*TransactionContext),
		listeners: make([]types.TransactionListener, 0),
		buffer:    make(map[string]*TransactionPair),
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
	// 为了保持消息有序，我们对请求和响应进行缓存
	txID := utils.BuildTransactionID(msg)
	pair, exists := m.buffer[txID]
	if !exists {
		pair = &TransactionPair{}
		m.buffer[txID] = pair
	}
	if msg.IsRequest() {
		pair.request = msg.(types.SipRequest)
	} else {
		pair.response = msg.(types.SipResponse)
	}
	pair.lastUopdateAt = time.Now()
	// 处理缓存中的请求和响应
	if pair.request != nil && pair.response != nil {
		// 如果请求和响应都存在，说明相关请求已收到，可以处理
		// 先处理请求，再处理响应
		err := m.HandleMessage0(pair.request)
		if err != nil {
			return err
		}
		err = m.HandleMessage0(pair.response)
		if err != nil {
			return err
		}
		// 处理完成后，删除缓存
		delete(m.buffer, txID)
	}
	// 如果不存在就等下次收到消息进行处理
	// 验证一下，我怀疑由于 partition 的存在，可能会导致请求和响应不在同一个 goroutine 中被处理，缓存需要设计成单例的
	return nil
}

func (m *TransactionManager) HandleMessage0(msg types.SipMessage) error {
	tx, exist := m.GetTransactionBySipMessage(msg)
	if !exist && msg.IsRequest() {
		if req, ok := msg.(types.SipRequest); ok {
			tx = m.CreateTransaction(req)
			if tx != nil {
				log.Logger.Infof("Created new transaction: %s", tx.ID())
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
				listener.OnTransactionTerminated(tx)
			}
		}
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
		if now.Sub(pair.lastUopdateAt) > expiredThreshold {
			delete(m.buffer, txID)
			log.Logger.WithField("Transaction-ID", txID).
				Infof("Cleared expired transaction pair, last updated: %s", pair.lastUopdateAt.Format(time.RFC3339))
		}
	}
}
