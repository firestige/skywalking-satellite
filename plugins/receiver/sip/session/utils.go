package sip

import (
	"fmt"
	"strings"

	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

// buildDialogID constructs a dialog ID based on the Call-ID, From-Tag, and To-Tag.
// If To-Tag is empty, it returns a boolean indicating whether the dialog ID is known.
// If To-Tag is not empty, it returns false, indicating that the dialog ID is not known.
// args:
//   - msg: SipMessage containing Call-ID, From-Tag, and To-Tag
//   - earlyDialog: true-early dialog, false-regular dialog
//
// returns:
//   - string: constructed dialog ID
func buildDialogID(msg types.SipMessage, earlyDialog bool) string {
	// 使用 Call-ID 和 From-Tag 作为事务 ID 的基础
	callID := msg.CallID()
	_, fromTag := extractURIAndTag(msg.From())
	_, toTag := extractURIAndTag(msg.To())
	if earlyDialog {
		return fmt.Sprintf("%s|%s", callID, fromTag)
	} else {
		return fmt.Sprintf("%s|%s|%s", callID, fromTag, toTag)
	}
}

func buildTransactionID(request types.SipMessage, forceUseKnown bool) string {
	// 使用 Call-ID 、 Cseq 和 Via头的branch 作为事务 ID 的基础(为了避免空格引起的问题，使用下划线替代CSeq中的空格)
	callID := request.CallID()
	cseq := strings.ReplaceAll(request.CSeq(), " ", "_")
	branch := request.ViaBranch()
	return fmt.Sprintf("%s|%s|%s", callID, cseq, branch)
}

func extractURIAndTag(header string) (string, string) {
	// 示例输入: "sip:alice@example.com;tag=12345" 或 "<sip:alice@example.com>;tag=12345"
	uri := ""
	tag := ""

	// 去除尖括号
	h := header
	if start := len(h); start > 0 && h[0] == '<' {
		if end := len(h); end > 0 && h[end-1] == '>' {
			h = h[1 : end-1]
		}
	}

	// 查找分号，分割参数
	semi := -1
	for i := 0; i < len(h); i++ {
		if h[i] == ';' {
			semi = i
			break
		}
	}
	if semi == -1 {
		// 没有参数，直接返回
		return h, ""
	}
	uri = h[:semi]
	params := h[semi+1:]

	// 查找tag参数
	for _, param := range splitParams(params) {
		if len(param) >= 4 && param[:4] == "tag=" {
			tag = param[4:]
			break
		}
	}
	return uri, tag
}

// 辅助函数：分割参数字符串
func splitParams(params string) []string {
	var result []string
	start := 0
	for i := 0; i < len(params); i++ {
		if params[i] == ';' {
			if start < i {
				result = append(result, params[start:i])
			}
			start = i + 1
		}
	}
	if start < len(params) {
		result = append(result, params[start:])
	}
	return result
}

func extractMethodFromCseq(cseq string) types.Method {
	// CSeq格式通常为 "1 INVITE" 或 "2 ACK"
	parts := strings.SplitN(cseq, " ", 2)
	if len(parts) < 2 {
		return types.MethodUnknown
	}
	method := strings.ToUpper(parts[1])
	switch method {
	case "INVITE":
		return types.Invite
	case "ACK":
		return types.Ack
	case "INFO":
		return types.Info
	case "BYE":
		return types.Bye
	case "CANCEL":
		return types.Cancel
	case "MESSAGE":
		return types.Message
	case "REFER":
		return types.Refer
	case "PRACK":
		return types.Prack
	case "UPDATE":
		return types.Update
	case "OPTIONS":
		return types.Options
	case "REGISTER":
		return types.Register
	case "SUBSCRIBE":
		return types.Subscribe
	case "NOTIFY":
		return types.Notify
	default:
		return types.MethodUnknown
	}
}
