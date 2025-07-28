package dialog

import (
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/utils"
)

type DialogManager struct {
	store map[string]*DialogContext // 使用 dialog-ID 作为对话标识
}

func NewDialogManager() *DialogManager {
	return &DialogManager{
		store: make(map[string]*DialogContext),
	}
}

func (dm *DialogManager) CreateDialog(req types.SipRequest) *DialogContext {
	ctx, err := NewDialogContext(req)
	if err != nil {
		log.Logger.WithError(err).Errorf("Failed to create dialog for request: %s", req.String())
		return nil // 如果创建对话失败，返回 nil
	}
	dm.store[ctx.ID()] = ctx
	return ctx
}

func (dm *DialogManager) GetDialogByCallID(callID string) (*DialogContext, bool) {
	ctx, exists := dm.store[callID]
	if !exists {
		return nil, false // 如果对话不存在，返回 nil 和 false
	}
	return ctx, true
}

func (dm *DialogManager) GetAllDialogs() []*DialogContext {
	var allDialogs []*DialogContext
	for _, ctx := range dm.store {
		allDialogs = append(allDialogs, ctx)
	}
	return allDialogs
}

func (dm *DialogManager) GetDialogBySipMessage(msg types.SipMessage) (*DialogContext, bool) {
	dialogID := utils.BuildDialogID(msg, false)
	ctx, exists := dm.store[dialogID]
	if !exists {
		dialogID := utils.BuildDialogID(msg, true)
		ctx, exists = dm.store[dialogID]
		if !exists {
			return nil, false // 如果对话不存在，返回 nil 和 false
		}
	}
	return ctx, true // 返回找到的对话上下文和 true
}

func (dm *DialogManager) HandleMessage(ctx *DialogContext, msg types.SipMessage) error {
	if ctx == nil {
		return nil // 如果上下文不存在，直接返回
	}
	return ctx.HandleMessage(msg)
}
