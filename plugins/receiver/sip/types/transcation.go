package types

type Dialog interface {
	ID() string
	State() DialogState
	SetState(state DialogState)
	UA() UA
	Transactions() []Transaction
	AddTransaction(tx Transaction)
	RemoveTransaction(txID string)
	CallID() string
	LocalTag() string
	RemoteTag() string
	LocalURI() string
	RemoteURI() string
	SessionType() SessionType
	CreatedAt() int64
	UpdatedAt() int64
	// 可选：属性扩展
	SetAttribute(key string, value interface{})
	GetAttribute(key string) (interface{}, bool)
}

type DialogManager interface {
	CreateDialog(callID, localTag, remoteTag, localURI, remoteURI string, sessionType SessionType) Dialog
	GetDialog(id string) Dialog
	UpdateDialog(dialog Dialog) error
	DeleteDialog(id string) error
	AddListener(name string, listener DialogListener)
	RemoveListener(name string, listener DialogListener)
	GetDialogs() []Dialog
	GetDialogsByCallID(callID string) []Dialog
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
	SetState(state TransactionState)
	DialogID() string
	Request() SipRequest
	LastResponse() SipResponse
	CreatedAt() int64
	UpdatedAt() int64
	Direction() UADirection // 新增：UAC/UAS
	IsTerminated() bool     // 新增
	Error() error           // 新增
}

type TransactionListener interface {
	OnTransactionCreated(tx Transaction)
	OnTransactionStateChanged(tx Transaction)
	OnTransactionTerminated(tx Transaction)
	OnTransactionTimeout(tx Transaction)
	OnTransactionError(tx Transaction, err error)
}

type TransactionManager interface {
	CreateTransaction(dialog Dialog, request SipRequest) Transaction
	GetTransaction(id string) Transaction
	UpdateTransaction(transaction Transaction) error
	DeleteTransaction(id string) error
	addListener(name string, listener TransactionListener)
	removeListener(name string, listener TransactionListener)
	GetTransactions() []Transaction
	GetTransactionsByDialog(dialogID string) []Transaction
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

type UA int

const (
	UAClient UA = iota
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

type UADirection int

const (
	UACDirection UADirection = iota
	UASDirection
)

type DialogEvent struct {
	Type      EventType
	Dialog    Dialog
	Timestamp int64
	Reason    string
}

type TransactionEvent struct {
	Type        EventType
	Transaction Transaction
	Timestamp   int64
	Error       error
}

type SessionType int

const (
	SessionTypeInvite SessionType = iota
	SessionTypeSubscribe
	SessionTypeNotify
	// ...
)
