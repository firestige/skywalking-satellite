package session

import (
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/utils"
)

type manager struct {
	serviceName        string
	instanceId         string
	segmentIdGenerator *utils.SegmentIDGenerator               // 用于生成Segment ID
	sessions           map[string]*types.Session               // 使用 CallID 作为键
	onRemoveSessionFns map[string]func(session *types.Session) // 存储 CallID 到删除函数的映射
}

func NewManager(serviceName, instanceId string) *manager {
	return &manager{
		serviceName:        serviceName,
		instanceId:         instanceId,
		segmentIdGenerator: utils.NewSegmentIDGenerator(instanceId),
		sessions:           make(map[string]*types.Session),
		onRemoveSessionFns: make(map[string]func(session *types.Session)),
	}
}

func (m *manager) GetOrCreateSession(msg *types.SipMessage) (*types.Session, error) {
	// 如果找到现会话，则返回
	// 如果没有找到，则创建一个新的会话并返回
	return nil, nil
}

func (m *manager) GetSession(msg *types.SipMessage) (*types.Session, bool) {
	return nil, false
}

func (m *manager) CreateSession(msg *types.SipMessage) (*types.Session, error) {
	// 1.创建一个新的会话
	// 	如果当前消息是会话外消息并且是响应，则返回 nil，因为不论出栈入栈，一定是从请求开始
	// 	如果当前消息是会话外消息并且是请求，则创建一个新的会话
	// 	如果当前消息是会话内消息并且不是起始消息，则返回 nil，因为不论出栈入栈，会话内消息一定是从起始请求开始
	//  如果当前消息是会话内消息并且是起始消息，则创建一个新的会话
	// 2.为新的会话，创建新的SegmentObject
	// 	traceId=if msg.headers().Get("X-ICC-Call-id") != "" msg.headers().Get("X-ICC-Call-id") or msg.callId()
	//  segmentId=segmentIdGenerator.GenerateSegmentID()
	//  service=serviceName
	//  instance=instanceId

	return nil, nil
}

func (m *manager) DoOnRemoveSession(callId string, fn func(session *types.Session)) {
	m.onRemoveSessionFns[callId] = fn
}

func (m *manager) Prepare() error {
	return nil
}

func (m *manager) Stop() {

}
