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

// 会话对象，主要通过callId和cseq来唯一标识一个SIP会话中的请求和响应
type Session struct {
	CallId      string // SIP Call ID
	CurrentCseq string // 当前 CSeq，Cseq一般由数字和方法组成，如 "1 INVITE"
	Segment     *agent.SegmentObject
	mu          sync.RWMutex // 保护并发访问
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
type SessionManager struct {
	sessions map[string]*Session // 使用 CallID 作为键
	mu       sync.RWMutex        // 保护并发访问
}

func (sm *SessionManager) Prepare() error {
	// 初始化会话管理器
	sm.sessions = make(map[string]*Session)
	return nil
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

// 创建新的 SIP 会话
// 需要根据请求的 Call ID 和 CSeq 创建新的会话对象
// 如果会话不存在，则创建新的会话
// 如果会话已存在，则更新其状态
// 返回新创建或更新的会话对象
// 可能需要处理并发问题，确保线程安全
// 需要考虑性能和资源管理，避免内存泄漏或过多的 goroutines
func (sm *SessionManager) createSession(req SipRequest) (*Session, error) {
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
				SpanId:    spanId,
				StartTime: time.Now().UnixMilli(),
				// 其他span属性需要根据具体需求设置
			}
			existingSession.Segment.Spans = append(existingSession.Segment.Spans, newSpan)
			existingSession.CurrentCseq = cseq

			log.Logger.Infof("Updated session for CallID: %s, new CSeq: %s", callId, cseq)
			return existingSession, nil
		} else if reqCSeqNum < currentCSeqNum {
			// CSeq的数字小于CurrentCSeq，返回已存在的会话对象，并生成一个error
			return existingSession, fmt.Errorf("CSeq number %d is less than current CSeq %d", reqCSeqNum, currentCSeqNum)
		} else {
			// CSeq相等，返回现有会话
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
	newSession := &Session{
		CallId:      callId,
		CurrentCseq: cseq,
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
			Spans: make([]*agent.SpanObject, 0),
		},
	}

	// 为spans添加新的spanObject，spanId设置为0
	initialSpan := &agent.SpanObject{
		SpanId:    0,
		StartTime: time.Now().UnixMilli(),
		// 其他span属性需要根据具体需求设置
	}
	newSession.Segment.Spans = append(newSession.Segment.Spans, initialSpan)

	// 将新会话添加到管理器中
	sm.sessions[callId] = newSession

	log.Logger.Infof("Created new session for CallID: %s, CSeq: %s", callId, cseq)
	return newSession, nil
}

func (sm *SessionManager) getSession(resp SipResponse) (*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	callId := resp.CallId()

	// 通过 Call ID 查找会话
	if session, exists := sm.sessions[callId]; exists {
		session.mu.RLock()
		defer session.mu.RUnlock()

		// 可以根据需要进一步验证CSeq匹配
		log.Logger.Infof("Found session for CallID: %s", callId)
		return session, nil
	}

	// 如果未找到会话，则返回 error
	return nil, fmt.Errorf("session not found for CallID: %s", callId)
}

// 清理会话的方法（可选，用于资源管理）
func (sm *SessionManager) removeSession(callId string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.sessions[callId]; exists {
		delete(sm.sessions, callId)
		log.Logger.Infof("Removed session for CallID: %s", callId)
	}
}

// 获取所有会话的方法（可选，用于调试和监控）
func (sm *SessionManager) getAllSessions() map[string]*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]*Session)
	for k, v := range sm.sessions {
		result[k] = v
	}
	return result
}
