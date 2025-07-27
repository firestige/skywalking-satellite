package sip

import (
	"time"

	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

type transaction struct {
	id        string
	txType    types.TransactionType
	txState   types.TransactionState
	createdAt int64
	updatedAt int64
	ua        types.UAType
	request   types.SipRequest
	response  types.SipResponse
	err       error // 错误信息，如果有的话
}

func NewTransaction(ua types.UAType, request types.SipRequest) *transaction {
	id := buildTransactionID(request, false)
	txType := createTransactionType(request)
	state := newTransactionState(txType, ua)
	return &transaction{
		id:        id,
		txType:    txType,
		txState:   state,
		ua:        ua,
		createdAt: time.Now().Unix(),
		updatedAt: time.Now().Unix(),
		request:   request,
	}
}

func createTransactionType(request types.SipRequest) types.TransactionType {
	switch request.Method() {
	case types.Invite, types.Ack, types.Bye, types.Cancel:
		return types.InviteTransactionType
	default:
		return types.NonInviteTransactionType
	}
}

func newTransactionState(txType types.TransactionType, ua types.UAType) types.TransactionState {
	switch ua {
	case types.UAClient:
		if txType == types.InviteTransactionType {
			return types.InviteTransactionStateCalling
		}
		return types.NonInviteTransactionStateTrying
	case types.UAServer:
		if txType == types.InviteTransactionType {
			return types.InviteTransactionStateProceeding
		}
		return types.NonInviteTransactionStateTrying
	default:
		return nil // 正常不会走到这里，可能是一个错误的状态
	}
}

func (t *transaction) ID() string {
	return t.id
}

func (t *transaction) Type() types.TransactionType {
	return t.txType
}

func (t *transaction) State() types.TransactionState {
	return t.txState
}

func (t *transaction) setState(state types.TransactionState) {
	t.txState = state
	t.updatedAt = time.Now().Unix()
}

func (t *transaction) UA() types.UAType {
	return t.ua
}

func (t *transaction) Request() types.SipRequest {
	return t.request
}

func (t *transaction) LastResponse() types.SipResponse {
	return t.response
}

func (t *transaction) CreatedAt() int64 {
	return t.createdAt
}

func (t *transaction) UpdatedAt() int64 {
	return t.updatedAt
}

func (t *transaction) IsTerminated() bool {
	return t.txState == types.InviteTransactionStateTerminated || t.txState == types.NonInviteTransactionStateTerminated
}

func (t *transaction) Error() error {
	return t.err
}
