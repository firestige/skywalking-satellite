package sip

import (
	"fmt"
	"time"

	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

type session struct {
	id          string
	dialogs     map[string]types.Dialog
	sessionType types.SessionType // 会话类型
	uaType      types.UAType
	createAt    int64
	updatedAt   int64
	metadatas   map[string]interface{} // 用于存储会话的元数据

	eventChan chan *types.SessionEvent // 用于处理会话事件
}

func NewSession(id string, sessionType types.SessionType, ua types.UAType, eventBus chan *types.SessionEvent) *session {
	return &session{
		id:          id,
		dialogs:     make(map[string]types.Dialog),
		sessionType: sessionType,
		uaType:      ua,
		eventChan:   eventBus,
		createAt:    time.Now().Unix(),
		updatedAt:   time.Now().Unix(),
		metadatas:   make(map[string]interface{}),
	}
}

func (s *session) ID() string {
	return s.id
}

func (s *session) Dialogs() map[string]types.Dialog {
	return s.dialogs
}

func (s *session) UAType() types.UAType {
	return s.uaType
}

func (s *session) SessionType() types.SessionType {
	return s.sessionType
}

func (s *session) CreateAt() int64 {
	return s.createAt
}

func (s *session) UpdatedAt() int64 {
	return s.updatedAt
}

func (s *session) AddDialog(dialog types.Dialog) {
	s.dialogs[dialog.ID()] = dialog
	s.updatedAt = time.Now().Unix()
}

func (s *session) RemoveDialog(dialogID string) {
	delete(s.dialogs, dialogID)
	s.updatedAt = time.Now().Unix()
}
func (s *session) GetDialog(dialogID string) types.Dialog {
	return s.dialogs[dialogID]
}

func (s *session) GetOrCreateDialogIfAbsent(msg types.SipMessage) (types.Dialog, error) {
	callID := msg.CallID()

	// 假设有extractURIAndTag工具函数
	_, fromTag := extractURIAndTag(msg.From())
	_, toTag := extractURIAndTag(msg.To())

	// 虚拟Dialog场景（如REGISTER/OPTIONS等）
	if s.hasVirtualDialog() {
		dialogID := buildDialogID(msg, false)
		if dialog, ok := s.dialogs[dialogID]; ok {
			return dialog, nil
		}
		// 只在请求时创建虚拟Dialog，响应找不到是异常
		if msg.IsRequest() {
			newDialog := NewDialog(dialogID, s, callID, fromTag, toTag)
			s.AddDialog(newDialog)
			return newDialog, nil
		}
		return nil, fmt.Errorf("response for virtual dialog not found: %s", dialogID)
	}

	// 非虚拟Dialog场景（如INVITE/SUBSCRIBE/NOTIFY）
	// 1. 请求消息（无ToTag，创建早期Dialog）
	if msg.IsRequest() && toTag == "" {
		dialogID := buildDialogID(msg, true)
		if _, ok := s.dialogs[dialogID]; ok {
			// 如果已经存在，说明链路有问题
			return nil, fmt.Errorf("early dialog already exists for %s", dialogID)
		}
		newDialog := NewDialog(dialogID, s, callID, fromTag, "")
		s.AddDialog(newDialog)
		return newDialog, nil
	}

	// 2. 响应或in-dialog请求（有ToTag，查找完整Dialog）
	dialogID := buildDialogID(msg, false)
	if dialog, ok := s.dialogs[dialogID]; ok {
		return dialog, nil
	}
	// 回退到早期Dialog（无ToTag），需要升级状态
	earlyDialogID := buildDialogID(msg, true)
	if earlyDialog, ok := s.dialogs[earlyDialogID]; ok {
		// 升级DialogID并状态
		delete(s.dialogs, earlyDialogID)
		earlyDialog.ChangeState(types.DialogStateEarly, types.DialogStateConfirmed)
		s.AddDialog(earlyDialog)
		return earlyDialog, nil
	}
	return nil, fmt.Errorf("dialog not found for %s", dialogID)
}

func (s *session) Metadatas() map[string]interface{} {
	// 实现具体的逻辑来返回元数据
	return make(map[string]interface{})
}

func (s *session) SetMetadata(key string, value interface{}) {
	// 实现具体的逻辑来设置元数据
}

func (s *session) GetMetadata(key string) (interface{}, bool) {
	// 实现具体的逻辑来获取元数据
	value, exists := s.Metadatas()[key]
	return value, exists
}

func (s *session) hasVirtualDialog() bool {
	switch s.sessionType {
	case types.SessionTypeInvite, types.SessionTypeSubscribe, types.SessionTypeNotify:
		// 如果是这几种类型的会话，说明没有虚拟Dialog，他们创建的dialog都是实际的
		return false
	default:
		// 其它类型的会话不会创建dialog，他们创建的dialog都是虚拟的
		return true
	}
}

func (s *session) onMessage(msg types.SipMessage) error {
	switch s.uaType {
	case types.UAServer:
		return s.handleUASMessage(msg)
	case types.UAClient:
		return s.handleUACMessage(msg)
	default:
		return fmt.Errorf("unknown UA type: %v", s.uaType)
	}
}

func (s *session) handleUASMessage(msg types.SipMessage) error {
	if req, ok := msg.(types.SipRequest); ok {
		return s.handleUASRequest(req)
	}
	if resp, ok := msg.(types.SipResponse); ok {
		return s.handleUASResponse(resp)
	}
	return fmt.Errorf("unsupported message type: %T", msg)
}

func (s *session) handleUASRequest(req types.SipRequest) error {
	// 处理UAS请求逻辑
	dialog, err := s.GetOrCreateDialogIfAbsent(req)
	if err != nil {
		return err
	}
	return dialog.handleUASRequest(req)
}

func (s *session) handleUASResponse(resp types.SipResponse) error {
	// 处理UAS响应逻辑
	dialog, err := s.GetOrCreateDialogIfAbsent(resp)
	if err != nil {
		return err
	}
	return dialog.handleUASResponse(resp)
}

func (s *session) handleUACMessage(msg types.SipMessage) error {
	if req, ok := msg.(types.SipRequest); ok {
		return s.handleUACRequest(req)
	}
	if resp, ok := msg.(types.SipResponse); ok {
		return s.handleUACResponse(resp)
	}
	return fmt.Errorf("unsupported message type: %T", msg)
}

func (s *session) handleUACRequest(req types.SipRequest) error {
	// 处理UAC请求逻辑
	dialog, err := s.GetOrCreateDialogIfAbsent(req)
	if err != nil {
		return err
	}
	return dialog.handleUACRequest(req)
}

func (s *session) handleUACResponse(resp types.SipResponse) error {
	// 处理UAC响应逻辑
	dialog, err := s.GetOrCreateDialogIfAbsent(resp)
	if err != nil {
		return err
	}
	return dialog.handleUACResponse(resp)
}
