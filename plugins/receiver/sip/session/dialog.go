package sip

import (
	"time"

	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

type dialog struct {
	id           string
	state        types.DialogState
	ua           types.UAType
	transactions map[string]types.Transaction // key: Transaction ID
	createAt     int64
	updatedAt    int64
	callID       string
	localURI     string
	localTag     string
	remoteURI    string
	remoteTag    string
}

func NewDialog(id string, ua types.UAType, callID, local, remote string) *dialog {
	localURI, localTag := extractURIAndTag(local)
	remoteURI, remoteTag := extractURIAndTag(remote)
	return &dialog{
		id:           id,
		state:        types.DialogStateEarly,
		ua:           ua,
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

func extractURIAndTag(header string) (string, string) {
	// 示例输入: "sip:alice@example.com;tag=12345" 或 "<sip:alice@example.com>;tag=12345"
	uri := ""
	tag := ""

	// 去除尖括号
	h := header
	if start := len(h); start > 0 && h[0] == '<' {
		if end := len(h); end > 0 && h[end-1] == '>' {
			h = h[1 : end-1]
		}
	}

	// 查找分号，分割参数
	semi := -1
	for i := 0; i < len(h); i++ {
		if h[i] == ';' {
			semi = i
			break
		}
	}
	if semi == -1 {
		// 没有参数，直接返回
		return h, ""
	}
	uri = h[:semi]
	params := h[semi+1:]

	// 查找tag参数
	for _, param := range splitParams(params) {
		if len(param) >= 4 && param[:4] == "tag=" {
			tag = param[4:]
			break
		}
	}
	return uri, tag
}

// 辅助函数：分割参数字符串
func splitParams(params string) []string {
	var result []string
	start := 0
	for i := 0; i < len(params); i++ {
		if params[i] == ';' {
			if start < i {
				result = append(result, params[start:i])
			}
			start = i + 1
		}
	}
	if start < len(params) {
		result = append(result, params[start:])
	}
	return result
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
	return d.ua
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

func (d *dialog) GetOrCreateTransactionIfAbsent(msg types.SipMessage) types.Transaction {
	// 构造事务ID的辅助函数
	buildTxID := func(callID, fromTag, toTag string) string {
		return callID + "|" + fromTag + "|" + toTag
	}

	callID := msg.CallID()
	fromTag := msg.FromTag()
	toTag := msg.ToTag()

	// in-dialog请求或响应（有ToTag）
	txID := buildTxID(callID, fromTag, toTag)
	if tx, ok := d.transactions[txID]; ok {
		return tx
	}

	// 响应但找不到事务，且Dialog为early，尝试用callID+fromTag查找
	if !msg.IsRequest() && d.state == types.DialogStateEarly {
		earlyTxID := buildTxID(callID, fromTag, "")
		if tx, ok := d.transactions[earlyTxID]; ok {
			// 用当前ToTag更新事务ID
			delete(d.transactions, earlyTxID)
			d.transactions[txID] = tx
			return tx
		}
	}

	// 非in-dialog请求或early响应，尝试用callID+fromTag查找
	if msg.IsRequest() && toTag == "" {
		earlyTxID := buildTxID(callID, fromTag, "")
		if tx, ok := d.transactions[earlyTxID]; ok {
			return tx
		}
		// 没有则创建新事务
		newTx := newTransaction(earlyTxID, msg)
		d.transactions[earlyTxID] = newTx
		return newTx
	}

	// 其它情况（如in-dialog请求但找不到事务，或响应但上下文缺失），创建新事务
	newTx := newTransaction(txID, msg)
	d.transactions[txID] = newTx
	return newTx
}
