package types

import "fmt"

type Session interface {
	ID() string
	Dialogs() map[string]Dialog
	SessionType() SessionType
	UAType() UAType
	CreateAt() int64
	UpdatedAt() int64
	AddDialog(dialog Dialog)
	RemoveDialog(dialogID string)
	GetDialog(dialogID string) Dialog
	GetOrCreateDialogIfAbsent(msg SipMessage) (Dialog, error)
	Metadatas() map[string]interface{}
	SetMetadata(key string, value interface{})
	GetMetadata(key string) (interface{}, bool)
}

type SessionType int

const (
	SessionTypeUnknown SessionType = iota
	SessionTypeInvite
	SessionTypeSubscribe
	SessionTypeNotify
	// ...
)

type SessionManager interface {
	Handle(msg SipMessage)
	GetSession(sessionID string) *Session
}

type Dialog interface {
	ID() string
	State() DialogState
	UA() UAType
	Transactions() map[string]Transaction
	AddTransaction(tx Transaction)
	RemoveTransaction(txID string)
	CallID() string
	LocalTag() string
	RemoteTag() string
	LocalURI() string
	RemoteURI() string
	CreatedAt() int64
	UpdatedAt() int64
	GetOrCreateTransactionIfAbsent(msg SipMessage) (Transaction, error)
	ChangeState(from, to DialogState) error
}

type DialogManager interface {
	// 创建新的Dialog
	// 如果已存在，则返回现有的Dialog
	// 如果不存在，则创建新的Dialog并返回
	CreateDialog(callID, localTag, remoteTag, localURI, remoteURI string, sessionType SessionType) Dialog
	// 获取Dialog
	// 如果没有找到，则直接返回nil
	// 如果有多个Dialog，则返回第一个找到的
	GetDialog(id string) Dialog
	// 更新Dialog状态
	UpdateDialog(dialog Dialog) error
	// 删除Dialog
	DeleteDialog(id string) error
	// 添加监听器
	AddListener(name string, listener DialogListener)
	// 移除监听器
	RemoveListener(name string, listener DialogListener)
	// 获取所有Dialog
	GetDialogs() []Dialog
	// 根据CallID和FromTag获取Dialog
	// 如果没有找到，则直接返回nil
	GetDialogsByCallID(callID string) []Dialog
	// 根据消息获取Dialog
	// 如果消息是请求，则根据CallID和FromTag获取
	// 如果消息是响应，则根据CallID、FromTag和ToTag获取
	// 如果没有找到，且不是in-dialog请求，则创建新的，如果是in-dialog请求则返回nil
	GetorCreateDialogByMessage(msg SipMessage) Dialog
}

type DialogListener interface {
	OnDialogCreated(dialog Dialog)
	OnDialogStateChanged(dialog Dialog)
	OnDialogTerminated(dialog Dialog)
}

type Transaction interface {
	ID() string
	Type() TransactionType
	State() TransactionState
	UA() UAType
	Request() SipRequest
	LastResponse() SipResponse
	CreatedAt() int64
	UpdatedAt() int64
	IsTerminated() bool // 新增
	Error() error
	// 变更状态                            // 新增
	ChangeState(from, to TransactionState) error // 新增
}

type TransactionListener interface {
	OnTransactionCreated(tx Transaction)
	OnTransactionStateChanged(tx Transaction)
	OnTransactionTerminated(tx Transaction)
	OnTransactionTimeout(tx Transaction)
	OnTransactionError(tx Transaction, err error)
}

type TransactionManager interface {
	// 创建新的Transaction
	// 如果已存在，则返回现有的Transaction
	// 如果不存在，则创建新的Transaction并返回
	CreateTransaction(dialog Dialog, request SipRequest) Transaction
	// 获取Transaction
	// 如果没有找到，则直接返回nil
	// 如果有多个Transaction，则返回第一个找到的
	GetTransaction(id string) Transaction
	// 更新Transaction状态
	UpdateTransaction(transaction Transaction) error
	// 删除Transaction
	DeleteTransaction(id string) error
	// 添加监听器
	addListener(name string, listener TransactionListener)
	// 移除监听器
	removeListener(name string, listener TransactionListener)
	// 获取所有Transaction
	GetTransactions() []Transaction
	// 根据Dialog ID获取所有Transaction
	// 如果没有找到，则直接返回空切片
	GetTransactionsByDialog(dialogID string) []Transaction
	// 根据消息获取Transaction，使用消息Via头的CallID+Cseq+branch构建key获取，
	// 如果是请求且没找到则创建新的，
	// 如果是响应且没找到则返回nil
	GetOrCreateTransactionByMessage(msg SipMessage) Transaction
}

type TransactionType int

const (
	InviteTransactionType TransactionType = iota
	NonInviteTransactionType
)

type InviteTransactionState int

const (
	InviteTransactionStateCalling InviteTransactionState = iota
	InviteTransactionStateProceeding
	InviteTransactionStateCompleted
	InviteTransactionStateConfirmed
	InviteTransactionStateTerminated
)

type NonInviteTransactionState int

const (
	NonInviteTransactionStateTrying NonInviteTransactionState = iota
	NonInviteTransactionStateProceeding
	NonInviteTransactionStateCompleted
	NonInviteTransactionStateTerminated
)

type DialogState int

const (
	DialogStateEarly DialogState = iota
	DialogStateConfirmed
	DialogStateTerminated
)

type UAType int

const (
	UAUnknown UAType = iota
	UAClient
	UAServer
)

type EventType int

const (
	// Dialog 相关事件
	EventDialogCreated      EventType = iota // Dialog 创建
	EventDialogStateChanged                  // Dialog 状态变化
	EventDialogTerminated                    // Dialog 终止/移除

	// Transaction 相关事件
	EventTransactionCreated      // Transaction 创建
	EventTransactionStateChanged // Transaction 状态变化
	EventTransactionTerminated   // Transaction 终止/移除
	EventTransactionTimeout      // Transaction 超时

	// 其它会话相关事件
	EventDialogTimeout // Dialog 超时
)

type TransactionState interface {
	IsTerminal() bool
	String() string
}

// InviteTransactionState 实现 TransactionState
func (s InviteTransactionState) IsTerminal() bool {
	return s == InviteTransactionStateTerminated
}
func (s InviteTransactionState) String() string {
	switch s {
	case InviteTransactionStateCalling:
		return "Calling"
	case InviteTransactionStateProceeding:
		return "Proceeding"
	case InviteTransactionStateCompleted:
		return "Completed"
	case InviteTransactionStateConfirmed:
		return "Confirmed"
	case InviteTransactionStateTerminated:
		return "Terminated"
	default:
		return "Unknown"
	}
}

// NonInviteTransactionState 实现 TransactionState
func (s NonInviteTransactionState) IsTerminal() bool {
	return s == NonInviteTransactionStateTerminated
}
func (s NonInviteTransactionState) String() string {
	switch s {
	case NonInviteTransactionStateTrying:
		return "Trying"
	case NonInviteTransactionStateProceeding:
		return "Proceeding"
	case NonInviteTransactionStateCompleted:
		return "Completed"
	case NonInviteTransactionStateTerminated:
		return "Terminated"
	default:
		return "Unknown"
	}
}

type SessionEvent struct {
	Type      EventType
	Session   Session
	Timestamp int64
	Error     error // 错误信息，如果有的话
}

var (
	ErrTransactionNotFound     = fmt.Errorf("transaction not found")
	ErrTransactionExists       = fmt.Errorf("transaction already exists")
	ErrInvalidTransactionState = fmt.Errorf("invalid transaction state")
)
