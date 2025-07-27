package sip

import (
	"fmt"

	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

func buildTransactionID(request types.SipRequest, forceUseKnown bool) string {
	// 使用 Call-ID 和 From-Tag 作为事务 ID 的基础
	callID := request.CallID()
	// 如果 From-Tag 不存在，则是错误状态，TODO: 处理这种情况
	fromTag := request.FromTag()
	// 如果 To-Tag 不存在，则使用 "unknown"
	toTag := request.ToTag()
	if toTag == "" || forceUseKnown {
		toTag = "unknown"
	}
	return fmt.Sprintf("%s|%s|%s", callID, fromTag, toTag)
}
