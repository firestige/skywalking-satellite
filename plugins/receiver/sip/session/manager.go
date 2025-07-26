package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

type manager struct {
	serviceName      string
	instanceId       string
	dialogs          map[string]*Dialog
	transactions     map[string]*Transaction
	dialogMutex      sync.RWMutex
	transactionMutex sync.RWMutex
	eventHandlers    map[SessionEventType][]SessionEventHandler
	sessionHandlers  map[SessionType]SessionHandler
	eventMutex       sync.RWMutex
}

func NewManager(serviceName, instanceId string) *manager {
	m := &manager{
		serviceName:     serviceName,
		instanceId:      instanceId,
		dialogs:         make(map[string]*Dialog),
		transactions:    make(map[string]*Transaction),
		eventHandlers:   make(map[SessionEventType][]SessionEventHandler),
		sessionHandlers: make(map[SessionType]SessionHandler),
	}

	// 注册默认会话处理器
	m.RegisterSessionHandler(&InviteSessionHandler{})
	m.RegisterSessionHandler(&RegisterSessionHandler{})

	return m
}

func (m *manager) GetOrCreateSession(msg types.SipMessage) (*types.Session, error) {
	dialogID := generateDialogID(msg)

	m.dialogMutex.Lock()
	defer m.dialogMutex.Unlock()

	if dialog, exists := m.dialogs[dialogID]; exists {
		return &types.Session{
			ID:     dialog.ID,
			CallID: dialog.CallID,
			State:  string(dialog.State),
			Type:   string(dialog.SessionType),
		}, nil
	}

	return m.createSession(msg)
}

func (m *manager) GetSession(msg types.SipMessage) (*types.Session, bool) {
	dialogID := generateDialogID(msg)

	m.dialogMutex.RLock()
	defer m.dialogMutex.RUnlock()

	if dialog, exists := m.dialogs[dialogID]; exists {
		return &types.Session{
			ID:     dialog.ID,
			CallID: dialog.CallID,
			State:  string(dialog.State),
			Type:   string(dialog.SessionType),
		}, true
	}

	return nil, false
}

func (m *manager) CreateSession(msg types.SipMessage) (*types.Session, error) {
	m.dialogMutex.Lock()
	defer m.dialogMutex.Unlock()

	return m.createSession(msg)
}

func (m *manager) createSession(msg types.SipMessage) (*types.Session, error) {
	if !msg.IsRequest() {
		return nil, fmt.Errorf("only SIP requests can create sessions")
	}
	req := msg.(types.SipRequest)
	sessionType := SessionType(req.Method())

	dialog := &Dialog{
		ID:          generateDialogID(req),
		CallID:      req.CallId(),
		LocalTag:    extractLocalTag(req),
		RemoteTag:   extractRemoteTag(req),
		LocalURI:    extractLocalURI(req),
		RemoteURI:   extractRemoteURI(req),
		LocalSeq:    extractLocalSeq(req),
		RemoteSeq:   extractRemoteSeq(req),
		State:       DialogStateEarly,
		SessionType: sessionType,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.dialogs[dialog.ID] = dialog

	// 创建事务
	transaction := &Transaction{
		ID:        generateTransactionID(req),
		DialogID:  dialog.ID,
		Method:    req.Method(),
		BranchID:  extractBranchID(req),
		State:     TransactionStateCalling,
		Request:   req,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.transactionMutex.Lock()
	m.transactions[transaction.ID] = transaction
	m.transactionMutex.Unlock()

	// 触发事件
	m.emitEvent(EventDialogCreated, dialog, transaction, req)
	m.emitEvent(EventTransactionStart, dialog, transaction, req)

	return &types.Session{
		ID:     dialog.ID,
		CallID: dialog.CallID,
		State:  string(dialog.State),
		Type:   string(dialog.SessionType),
	}, nil
}

// HandleMessage 处理SIP消息
func (m *manager) HandleMessage(msg types.SipMessage) error {
	dialogID := generateDialogID(msg)

	m.dialogMutex.RLock()
	dialog, dialogExists := m.dialogs[dialogID]
	m.dialogMutex.RUnlock()

	if !dialogExists {
		// 创建新会话
		_, err := m.CreateSession(msg)
		return err
	}

	// 获取或创建事务
	transactionID := generateTransactionID(msg)

	m.transactionMutex.Lock()
	transaction, transactionExists := m.transactions[transactionID]
	if !transactionExists {
		transaction = &Transaction{
			ID:        transactionID,
			DialogID:  dialog.ID,
			Method:    msg.(types.SipRequest).Method(),
			BranchID:  extractBranchID(msg),
			State:     TransactionStateCalling,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.transactions[transactionID] = transaction
		m.emitEvent(EventTransactionStart, dialog, transaction, msg)
	}
	m.transactionMutex.Unlock()

	// 根据消息类型处理
	if msg.IsRequest() {
		return m.handleRequest(dialog, transaction, msg.(types.SipRequest))
	} else {
		return m.handleResponse(dialog, transaction, msg.(types.SipResponse))
	}
}

func (m *manager) handleRequest(dialog *Dialog, transaction *Transaction, msg types.SipRequest) error {
	// 更新事务状态
	transaction.Request = msg
	transaction.UpdatedAt = time.Now()

	// 获取会话处理器
	if handler, exists := m.sessionHandlers[dialog.SessionType]; exists {
		return handler.HandleRequest(dialog, transaction, msg)
	}

	return fmt.Errorf("no handler for session type: %s", dialog.SessionType)
}

func (m *manager) handleResponse(dialog *Dialog, transaction *Transaction, msg types.SipResponse) error {
	// 更新事务状态
	transaction.Response = msg
	transaction.UpdatedAt = time.Now()

	// 根据响应码更新对话状态
	statusCode := msg.Status()

	// 根据会话类型处理不同的状态转换
	if dialog.SessionType == SessionTypeInvite {
		// INVITE会话有完整的三种状态
		switch {
		case statusCode >= 100 && statusCode < 200:
			// 1xx响应，保持早期对话状态
			if dialog.State != DialogStateEarly {
				dialog.State = DialogStateEarly
				m.emitEvent(EventDialogEarly, dialog, transaction, msg)
			}
			transaction.State = TransactionStateProceeding
		case statusCode >= 200 && statusCode < 300:
			// 2xx响应，确认对话
			if dialog.State != DialogStateConfirmed {
				dialog.State = DialogStateConfirmed
				dialog.RemoteTag = extractRemoteTag(msg)
				m.emitEvent(EventDialogConfirmed, dialog, transaction, msg)
			}
			transaction.State = TransactionStateCompleted
		case statusCode >= 300:
			// 3xx及以上响应，终止对话
			if dialog.State != DialogStateTerminated {
				dialog.State = DialogStateTerminated
				m.emitEvent(EventDialogTerminated, dialog, transaction, msg)
			}
			transaction.State = TransactionStateCompleted
		}
	} else {
		// 非INVITE会话（REGISTER、OPTIONS等）只有两种状态：Early和Terminated
		switch {
		case statusCode >= 100 && statusCode < 200:
			// 1xx响应，临时响应状态
			if dialog.State != DialogStateEarly {
				dialog.State = DialogStateEarly
				m.emitEvent(EventDialogEarly, dialog, transaction, msg)
			}
			transaction.State = TransactionStateProceeding
		case statusCode >= 200 && statusCode < 400:
			// 2xx和3xx响应，直接终止对话（成功或重定向）
			if dialog.State != DialogStateTerminated {
				dialog.State = DialogStateTerminated
				if statusCode >= 200 && statusCode < 300 {
					// 成功响应时更新远程标签
					dialog.RemoteTag = extractRemoteTag(msg)
				}
				m.emitEvent(EventDialogTerminated, dialog, transaction, msg)
			}
			transaction.State = TransactionStateCompleted
		case statusCode >= 400:
			// 4xx及以上响应，错误终止对话
			if dialog.State != DialogStateTerminated {
				dialog.State = DialogStateTerminated
				m.emitEvent(EventDialogTerminated, dialog, transaction, msg)
			}
			transaction.State = TransactionStateCompleted
		}
	}

	dialog.UpdatedAt = time.Now()

	// 获取会话处理器
	if handler, exists := m.sessionHandlers[dialog.SessionType]; exists {
		return handler.HandleResponse(dialog, transaction, msg)
	}

	return fmt.Errorf("no handler for session type: %s", dialog.SessionType)
}

// RegisterEventHandler 注册事件处理器
func (m *manager) RegisterEventHandler(eventType SessionEventType, handler SessionEventHandler) {
	m.eventMutex.Lock()
	defer m.eventMutex.Unlock()

	m.eventHandlers[eventType] = append(m.eventHandlers[eventType], handler)
}

// RegisterSessionHandler 注册会话处理器
func (m *manager) RegisterSessionHandler(handler SessionHandler) {
	m.sessionHandlers[handler.GetSessionType()] = handler
}

// emitEvent 触发事件
func (m *manager) emitEvent(eventType SessionEventType, dialog *Dialog, transaction *Transaction, msg types.SipMessage) {
	m.eventMutex.RLock()
	handlers, exists := m.eventHandlers[eventType]
	m.eventMutex.RUnlock()

	if !exists {
		return
	}

	event := &SessionEvent{
		Type:        eventType,
		Dialog:      dialog,
		Transaction: transaction,
		Message:     msg,
		Timestamp:   time.Now(),
	}

	for _, handler := range handlers {
		go func(h SessionEventHandler) {
			_ = h(event)
		}(handler)
	}
}

// 清理和维护方法
func (m *manager) Prepare() error {
	// 启动清理定时器等初始化工作
	go m.startCleanupTimer()
	return nil
}

func (m *manager) Stop() {
	// 清理资源
	m.dialogMutex.Lock()
	m.dialogs = make(map[string]*Dialog)
	m.dialogMutex.Unlock()

	m.transactionMutex.Lock()
	m.transactions = make(map[string]*Transaction)
	m.transactionMutex.Unlock()
}

// startCleanupTimer 启动清理定时器
func (m *manager) startCleanupTimer() {
	ticker := time.NewTicker(time.Minute * 5)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupExpiredSessions()
	}
}

// cleanupExpiredSessions 清理过期会话
func (m *manager) cleanupExpiredSessions() {
	now := time.Now()
	expireTime := time.Hour * 24 // 24小时过期

	m.dialogMutex.Lock()
	for id, dialog := range m.dialogs {
		if now.Sub(dialog.UpdatedAt) > expireTime || dialog.State == DialogStateTerminated {
			delete(m.dialogs, id)
		}
	}
	m.dialogMutex.Unlock()

	m.transactionMutex.Lock()
	for id, transaction := range m.transactions {
		if now.Sub(transaction.UpdatedAt) > time.Minute*5 || transaction.State == TransactionStateTerminated {
			delete(m.transactions, id)
		}
	}
	m.transactionMutex.Unlock()
}
