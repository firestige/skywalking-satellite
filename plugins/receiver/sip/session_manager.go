package sip

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/hashicorp/go-uuid"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

const (
	// 默认会话清理配置
	DefaultSessionTTL             = 30 * time.Minute // 默认会话过期时间
	DefaultSessionCleanupInterval = 5 * time.Minute  // 默认清理检查间隔
)

// SessionManagerConfig 会话管理器配置
type SessionManagerConfig struct {
	ServiceName     string        // 服务名称
	ServiceInstance string        // 服务实例名称
	SessionTTL      time.Duration // 会话过期时间
	CleanupInterval time.Duration // 清理检查间隔
}

// SessionManager 管理 SIP 会话
// 负责创建、获取和维护 SIP 会话
// 处理请求和响应的匹配
// 以及会话的生命周期管理
// 可能需要处理超时、清理等逻辑
// 需要与 ContextCounterManager 协同工作
// 以确保 traceID 的上下文计数器正确更新
// 需要处理并发安全问题，可能需要使用锁或其他同步机制
// 需要考虑性能和资源管理，避免内存泄漏或过多的 goroutines
// 需要提供 Prepare 方法以便在启动前进行必要的初始化
type sessionManager struct {
	sessions           map[string]*Session               // 使用 CallID 作为键
	onRemoveSessionFns map[string]func(session *Session) // 存储 CallID 到删除函数的映射
	mu                 sync.RWMutex                      // 保护并发访问
	config             SessionManagerConfig
	cleanupTicker      *time.Ticker
	stopCleanupChan    chan struct{}
}

// NewSessionManager 创建新的会话管理器
func NewSessionManager(config SessionManagerConfig) SipSessionManager {
	// 设置默认值
	if config.SessionTTL == 0 {
		config.SessionTTL = DefaultSessionTTL
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = DefaultSessionCleanupInterval
	}

	return &sessionManager{
		sessions:        make(map[string]*Session),
		config:          config,
		stopCleanupChan: make(chan struct{}),
	}
}

func (sm *sessionManager) Prepare() error {
	// 初始化会话管理器
	sm.sessions = make(map[string]*Session)

	// 启动自动清理 goroutine
	sm.startAutoCleanup()

	log.Logger.Infof("SessionManager prepared with TTL: %v, cleanup interval: %v",
		sm.config.SessionTTL, sm.config.CleanupInterval)

	return nil
}

// startAutoCleanup 启动自动清理机制
func (sm *sessionManager) startAutoCleanup() {
	sm.cleanupTicker = time.NewTicker(sm.config.CleanupInterval)

	go func() {
		defer sm.cleanupTicker.Stop()

		for {
			select {
			case <-sm.cleanupTicker.C:
				sm.performCleanup()
			case <-sm.stopCleanupChan:
				log.Logger.Info("SessionManager cleanup goroutine stopped")
				return
			}
		}
	}()

	log.Logger.Info("SessionManager auto-cleanup started")
}

// performCleanup 执行会话清理
func (sm *sessionManager) performCleanup() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	expiredSessions := make([]string, 0)

	// 查找过期的会话
	for callId, session := range sm.sessions {
		session.mu.RLock()
		lastModified := session.LastModified
		session.mu.RUnlock()

		if now.Sub(lastModified) > sm.config.SessionTTL {
			expiredSessions = append(expiredSessions, callId)
		}
	}

	// 删除过期的会话
	for _, callId := range expiredSessions {
		delete(sm.sessions, callId)
		fn := sm.onRemoveSessionFns[callId]
		if fn != nil {
			fn(nil) // 执行删除函数，传入 nil 表示会话已删除
		}
		log.Logger.Infof("Cleaned up expired session for CallID: %s", callId)
	}

	if len(expiredSessions) > 0 {
		log.Logger.Infof("Cleaned up %d expired sessions", len(expiredSessions))
	}
}

// Stop 停止会话管理器
func (sm *sessionManager) Stop() {
	if sm.stopCleanupChan != nil {
		close(sm.stopCleanupChan)
	}

	if sm.cleanupTicker != nil {
		sm.cleanupTicker.Stop()
	}

	log.Logger.Info("SessionManager stopped")
}

// updateSessionLastModified 更新会话的最后修改时间
func (sm *sessionManager) updateSessionLastModified(session *Session) {
	session.mu.Lock()
	defer session.mu.Unlock()
	session.LastModified = time.Now()
}

// 检查是否为会话内方法
func isInSessionMethod(method string) bool {
	inSessionMethods := []string{"ACK", "CANCEL", "BYE", "INFO"}
	for _, m := range inSessionMethods {
		if strings.EqualFold(method, m) {
			return true
		}
	}
	return false
}

// 从CSeq中提取序号
func extractCSeqNumber(cseq string) (int, error) {
	parts := strings.Fields(cseq)
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid CSeq format: %s", cseq)
	}
	return strconv.Atoi(parts[0])
}

// 从CSeq中提取方法
func extractCSeqMethod(cseq string) (string, error) {
	parts := strings.Fields(cseq)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid CSeq format: %s", cseq)
	}
	return parts[1], nil
}

func (sm *sessionManager) GetOrCreateSession(msg SipMessage) (*Session, error) {
	switch m := msg.(type) {
	case SipRequest:
		return sm.createSession(m)
	case SipResponse:
		return sm.getSession(m)
	default:
		return nil, fmt.Errorf("unsupported SIP message type: %T", msg)
	}
}

// 创建新的 SIP 会话
// 需要根据请求的 Call ID 和 CSeq 创建新的会话对象
// 如果会话不存在，则创建新的会话
// 如果会话已存在，则更新其状态
// 返回新创建或更新的会话对象
// 可能需要处理并发问题，确保线程安全
// 需要考虑性能和资源管理，避免内存泄漏或过多的 goroutines
func (sm *sessionManager) createSession(req SipRequest) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	callId := req.CallId()
	cseq := req.CSeq()

	// 首先通过call-id查找是否已经存在会话
	if existingSession, exists := sm.sessions[callId]; exists {
		existingSession.mu.Lock()
		defer existingSession.mu.Unlock()

		// 提取当前CSeq和请求CSeq的序号
		currentCSeqNum, err := extractCSeqNumber(existingSession.CurrentCseq)
		if err != nil {
			log.Logger.Errorf("Failed to extract current CSeq number: %v", err)
			return existingSession, fmt.Errorf("invalid current CSeq format")
		}

		reqCSeqNum, err := extractCSeqNumber(cseq)
		if err != nil {
			log.Logger.Errorf("Failed to extract request CSeq number: %v", err)
			return existingSession, fmt.Errorf("invalid request CSeq format")
		}

		if reqCSeqNum > currentCSeqNum {
			// CSeq的数字大于CurrentCSeq，在session的segment中为spans添加新的spanObject
			spanId := int32(len(existingSession.Segment.Spans))
			newSpan := &agent.SpanObject{
				SpanId: spanId,
				// 其他span属性需要根据具体需求设置
			}
			existingSession.Segment.Spans = append(existingSession.Segment.Spans, newSpan)
			existingSession.CurrentCseq = cseq
			existingSession.CurrentSpan = spanId      // 更新当前span ID
			existingSession.LastModified = time.Now() // 更新最后修改时间

			log.Logger.Infof("Updated session for CallID: %s, new CSeq: %s", callId, cseq)
			return existingSession, nil
		} else if reqCSeqNum < currentCSeqNum {
			// CSeq的数字小于CurrentCSeq，返回已存在的会话对象，并生成一个error
			// 但仍然更新最后修改时间，因为有活动
			existingSession.LastModified = time.Now()
			return existingSession, fmt.Errorf("CSeq number %d is less than current CSeq %d", reqCSeqNum, currentCSeqNum)
		} else {
			// CSeq相等，更新最后修改时间并返回现有会话
			existingSession.LastModified = time.Now()
			return existingSession, nil
		}
	}

	// 如果不存在，则判断消息的CSeq是否为会话内方法
	method, err := extractCSeqMethod(cseq)
	if err != nil {
		return nil, fmt.Errorf("failed to extract method from CSeq: %v", err)
	}

	if isInSessionMethod(method) {
		// 如果是会话内方法，则返回error
		return nil, fmt.Errorf("cannot create session with in-session method: %s", method)
	}

	// 创建新的会话对象
	now := time.Now()
	newSession := &Session{
		CallId:       callId,
		CurrentCseq:  cseq,
		CurrentSpan:  0, // 初始span ID为0
		LastModified: now,
		Segment: &agent.SegmentObject{
			TraceId: req.CallId(), // 需要从请求中获取或生成
			TraceSegmentId: func() string {
				id, err := uuid.GenerateUUID()
				if err != nil {
					log.Logger.Errorf("Failed to generate UUID for SegmentId: %v", err)
					return ""
				}
				return id
			}(), // 需要从请求中获取或生成
			Service:         sm.config.ServiceName,
			ServiceInstance: sm.config.ServiceInstance,
			Spans:           make([]*agent.SpanObject, 0),
		},
	}

	// 为spans添加新的spanObject，spanId设置为0
	initialSpan := &agent.SpanObject{
		SpanId: 0,
		// 其他span属性需要根据具体需求设置
	}
	newSession.Segment.Spans = append(newSession.Segment.Spans, initialSpan)

	// 将新会话添加到管理器中
	sm.sessions[callId] = newSession

	log.Logger.Infof("Created new session for CallID: %s, CSeq: %s", callId, cseq)
	return newSession, nil
}

func (sm *sessionManager) getSession(resp SipResponse) (*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	callId := resp.CallId()

	// 通过 Call ID 查找会话
	if session, exists := sm.sessions[callId]; exists {
		session.mu.Lock()
		defer session.mu.Unlock()

		// 更新最后修改时间
		session.LastModified = time.Now()

		// 可以根据需要进一步验证CSeq匹配
		log.Logger.Infof("Found session for CallID: %s", callId)
		return session, nil
	}

	// 如果未找到会话，则返回 error
	return nil, fmt.Errorf("session not found for CallID: %s", callId)
}

func (sm *sessionManager) DoOnRemoveSession(callId string, fn func(session *Session)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onRemoveSessionFns[callId] = fn
	log.Logger.Infof("Registered onRemoveSession function for CallID: %s", callId)
}

// 清理会话的方法（可选，用于资源管理）
func (sm *sessionManager) removeSession(callId string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.sessions[callId]; exists {
		delete(sm.sessions, callId)
		log.Logger.Infof("Removed session for CallID: %s", callId)
	}
}

// 获取所有会话的方法（可选，用于调试和监控）
func (sm *sessionManager) getAllSessions() map[string]*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]*Session)
	for k, v := range sm.sessions {
		result[k] = v
	}
	return result
}

// GetStats 获取会话管理器统计信息
func (sm *sessionManager) GetStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := map[string]interface{}{
		"active_sessions":  len(sm.sessions),
		"session_ttl":      sm.config.SessionTTL.String(),
		"cleanup_interval": sm.config.CleanupInterval.String(),
		"timestamp":        time.Now().Unix(),
	}

	return stats
}
