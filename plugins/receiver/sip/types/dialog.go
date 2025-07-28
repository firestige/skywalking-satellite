package types

type Dialog interface {
	ID() string
	UA() UAType
	CallID() string
	LocalTag() string
	RemoteTag() string
	LocalURI() string
	RemoteURI() string
	CreatedAt() int64
	UpdatedAt() int64
	Metadatas() map[string]string
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
