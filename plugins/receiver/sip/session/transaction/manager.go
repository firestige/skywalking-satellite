package transaction

import (
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/utils"
)

type TransactionManager struct {
	store map[string]*TransactionContext // 使用事务ID作为标识
}

func NewTransactionManager() *TransactionManager {
	return &TransactionManager{
		store: make(map[string]*TransactionContext),
	}
}

func (m *TransactionManager) CreateTransaction(msg types.SipMessage) *TransactionContext {
	if req, ok := msg.(types.SipRequest); ok {
		ua := utils.ParseUAType(req)
		switch req.Method() {
		case types.MethodInvite:
			// 对于Invite请求，客户端和服务器端的处理逻辑不同
			if ua == types.UAClient {
				return NewTransaction(req, &InviteCallingState{})
			}
			if ua == types.UAServer {
				return NewTransaction(req, &InviteProceedingState{})
			}
		default:
			return NewTransaction(req, &NonInviteTryingState{})
		}
	}
	return nil
}

func (m *TransactionManager) HandleMessage(ctx *TransactionContext, msg types.SipMessage) error {
	return ctx.HandleMessage(msg)
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
