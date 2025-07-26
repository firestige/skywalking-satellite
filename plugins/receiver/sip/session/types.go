package session

import (
	"time"

	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

// SessionType 会话类型枚举
type SessionType string

const (
	SessionTypeInvite    SessionType = "INVITE"
	SessionTypeRegister  SessionType = "REGISTER"
	SessionTypeSubscribe SessionType = "SUBSCRIBE"
	SessionTypeMessage   SessionType = "MESSAGE"
	SessionTypeOptions   SessionType = "OPTIONS"
)

// DialogState 对话状态
type DialogState string

const (
	DialogStateEarly      DialogState = "EARLY"      // 早期对话
	DialogStateConfirmed  DialogState = "CONFIRMED"  // 确认对话
	DialogStateTerminated DialogState = "TERMINATED" // 终止对话
)

// TransactionState 事务状态
type TransactionState string

const (
	TransactionStateCalling    TransactionState = "CALLING"
	TransactionStateProceeding TransactionState = "PROCEEDING"
	TransactionStateCompleted  TransactionState = "COMPLETED"
	TransactionStateConfirmed  TransactionState = "CONFIRMED"
	TransactionStateTerminated TransactionState = "TERMINATED"
)

// Dialog 表示SIP对话
type Dialog struct {
	ID          string
	CallID      string
	LocalTag    string
	RemoteTag   string
	LocalURI    string
	RemoteURI   string
	LocalSeq    uint32
	RemoteSeq   uint32
	State       DialogState
	SessionType SessionType
	CreatedAt   time.Time
	UpdatedAt   time.Time
	RouteSet    []string
}

// Transaction 表示SIP事务
type Transaction struct {
	ID        string
	DialogID  string
	Method    string
	BranchID  string
	State     TransactionState
	Request   types.SipRequest
	Response  types.SipResponse
	CreatedAt time.Time
	UpdatedAt time.Time
	Timer     *time.Timer
}

// SessionEvent 会话事件
type SessionEvent struct {
	Type        SessionEventType
	Dialog      *Dialog
	Transaction *Transaction
	Message     types.SipMessage
	Timestamp   time.Time
}

// SessionEventType 会话事件类型
type SessionEventType string

const (
	EventDialogCreated    SessionEventType = "DIALOG_CREATED"
	EventDialogEarly      SessionEventType = "DIALOG_EARLY"
	EventDialogConfirmed  SessionEventType = "DIALOG_CONFIRMED"
	EventDialogTerminated SessionEventType = "DIALOG_TERMINATED"
	EventTransactionStart SessionEventType = "TRANSACTION_START"
	EventTransactionEnd   SessionEventType = "TRANSACTION_END"
)

// SessionEventHandler 会话事件处理器

// SessionHandler 会话处理器接口
type SessionHandler interface {
	HandleRequest(dialog *Dialog, transaction *Transaction, msg types.SipRequest) error
	HandleResponse(dialog *Dialog, transaction *Transaction, msg types.SipResponse) error
	GetSessionType() SessionType
}
