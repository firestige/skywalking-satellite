package sip

import (
	"time"

	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

type session struct {
	id          string
	dialogs     map[string]types.Dialog
	sessionType types.SessionType // 会话类型
	createAt    int64
	updatedAt   int64
	metadatas   map[string]interface{} // 用于存储会话的元数据
}

func NewSession(id string, sessionType types.SessionType) *session {
	return &session{
		id:          id,
		dialogs:     make(map[string]types.Dialog),
		sessionType: sessionType,
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

func (s *session) GetOrCreateDialogIfAbsent(msg types.SipMessage) types.Dialog {
	dialogID := msg.CallID() + "|" + msg.FromTag() + "|" + msg.ToTag()
	if dialog, exists := s.dialogs[dialogID]; exists {
		return dialog
	}
	// 创建新的Dialog
	newDialog := NewDialog()
	s.AddDialog(newDialog)
	return newDialog
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
