package sip

import (
	"fmt"
	"sync"
	"time"

	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

type dialog struct {
	id           string
	state        types.DialogState
	uaType       types.UAType
	transactions map[string]types.Transaction // key: Transaction ID
	createAt     int64
	updatedAt    int64
	callID       string
	localURI     string
	localTag     string
	remoteURI    string
	remoteTag    string
	session      types.Session // 关联的会话

	mu *sync.RWMutex
}

func NewDialog(id string, session types.Session, callID, local, remote string) *dialog {
	localURI, localTag := extractURIAndTag(local)
	remoteURI, remoteTag := extractURIAndTag(remote)
	return &dialog{
		id:           id,
		state:        types.DialogStateEarly,
		uaType:       session.UAType(),
		session:      session,
		transactions: make(map[string]types.Transaction),
		createAt:     time.Now().Unix(),
		updatedAt:    time.Now().Unix(),
		callID:       callID,
		localTag:     localTag,
		remoteTag:    remoteTag,
		localURI:     localURI,
		remoteURI:    remoteURI,
	}
}

func (d *dialog) ID() string {
	return d.id
}

func (d *dialog) State() types.DialogState {
	return d.state
}

func (d *dialog) setState(state types.DialogState) {
	d.state = state
	d.updatedAt = time.Now().Unix()
}

func (d *dialog) UA() types.UAType {
	return d.uaType
}

func (d *dialog) Transactions() map[string]types.Transaction {
	return d.transactions
}

func (d *dialog) AddTransaction(tx types.Transaction) {
	d.transactions[tx.ID()] = tx
	d.updatedAt = time.Now().Unix()
}

func (d *dialog) RemoveTransaction(txID string) {
	delete(d.transactions, txID)
	d.updatedAt = time.Now().Unix()
}

func (d *dialog) CallID() string {
	return d.callID
}

func (d *dialog) LocalTag() string {
	return d.localTag
}

func (d *dialog) RemoteTag() string {
	return d.remoteTag
}

func (d *dialog) LocalURI() string {
	return d.localURI
}

func (d *dialog) RemoteURI() string {
	return d.remoteURI
}

func (d *dialog) CreatedAt() int64 {

	return d.createAt
}

func (d *dialog) UpdatedAt() int64 {
	return d.updatedAt
}

func (d *dialog) GetOrCreateTransactionIfAbsent(msg types.SipMessage) (types.Transaction, error) {
	txId := buildTransactionID(msg, false)
	if tx, ok := d.transactions[txId]; ok {
		return tx, nil
	} else if msg.IsRequest() {
		tx := NewTransaction(d, msg.(types.SipRequest))
		d.transactions[txId] = tx
		return tx, nil
	} else {
		// 响应不能找不到事务，应该返回错误
		return nil, fmt.Errorf("Unable to find the transaction for response message: %s", msg.String())
	}
}

func (d *dialog) ChangeState(from, to types.DialogState) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.state != from {
		return fmt.Errorf("dialog state change error: expected %s, got %s", from, d.state)
	}
	d.setState(to)
	return nil
}
