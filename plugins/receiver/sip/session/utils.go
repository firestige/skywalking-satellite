package session

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

// generateDialogID 生成对话ID
func generateDialogID(msg types.SipMessage) string {
	data := fmt.Sprintf("%s-%s-%s", msg.CallId(), extractLocalTag(msg), extractRemoteTag(msg))
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// generateTransactionID 生成事务ID
func generateTransactionID(msg types.SipMessage) string {
	// RFC3261: 事务由Via头部的branch参数标识
	branchID := extractBranchID(msg)

	if branchID == "" {
		// 如果没有branch参数（可能是老版本SIP），回退到传统方式
		// 使用Call-ID + CSeq生成，CSeq已包含序号和方法
		data := fmt.Sprintf("%s-%s", msg.CallId(), msg.Headers()["CSeq"])
		hash := md5.Sum([]byte(data))
		return hex.EncodeToString(hash[:])
	}

	// 标准情况下，branch参数就是事务的唯一标识
	return branchID
}

// extractLocalTag 提取本地标签
func extractLocalTag(msg types.SipMessage) string {
	// 在Proxy模式下，本地/远程的概念需要重新定义
	// 基于Connection.Direction来判断消息方向
	headers := msg.Headers()
	conn := msg.Connection()

	if conn == nil {
		// 无连接信息时，回退到传统判断
		if msg.IsRequest() {
			if from, exists := headers["From"]; exists {
				return parseTagFromHeader(from)
			}
		} else {
			if to, exists := headers["To"]; exists {
				return parseTagFromHeader(to)
			}
		}
		return ""
	}

	// 基于连接方向判断本地标签
	if conn.Direction == types.DirectionInbound {
		// 入栈消息：To头部代表本地端
		if to, exists := headers["To"]; exists {
			return parseTagFromHeader(to)
		}
	} else {
		// 出栈消息：From头部代表本地端
		if from, exists := headers["From"]; exists {
			return parseTagFromHeader(from)
		}
	}

	return ""
}

// extractRemoteTag 提取远程标签
func extractRemoteTag(msg types.SipMessage) string {
	// 与extractLocalTag相反的逻辑
	headers := msg.Headers()
	conn := msg.Connection()

	if conn == nil {
		// 无连接信息时，回退到传统判断
		if msg.IsRequest() {
			if to, exists := headers["To"]; exists {
				return parseTagFromHeader(to)
			}
		} else {
			if from, exists := headers["From"]; exists {
				return parseTagFromHeader(from)
			}
		}
		return ""
	}

	// 基于连接方向判断远程标签
	if conn.Direction == types.DirectionInbound {
		// 入栈消息：From头部代表远程端
		if from, exists := headers["From"]; exists {
			return parseTagFromHeader(from)
		}
	} else {
		// 出栈消息：To头部代表远程端
		if to, exists := headers["To"]; exists {
			return parseTagFromHeader(to)
		}
	}

	return ""
}

// extractLocalURI 提取本地URI
func extractLocalURI(msg types.SipMessage) string {
	// 基于连接方向判断本地URI
	headers := msg.Headers()
	conn := msg.Connection()

	if conn == nil {
		// 无连接信息时，回退到传统判断
		if msg.IsRequest() {
			if from, exists := headers["From"]; exists {
				return parseURIFromHeader(from)
			}
		} else {
			if to, exists := headers["To"]; exists {
				return parseURIFromHeader(to)
			}
		}
		return ""
	}

	// 基于连接方向判断本地URI
	if conn.Direction == types.DirectionInbound {
		// 入栈消息：To头部代表本地端
		if to, exists := headers["To"]; exists {
			return parseURIFromHeader(to)
		}
	} else {
		// 出栈消息：From头部代表本地端
		if from, exists := headers["From"]; exists {
			return parseURIFromHeader(from)
		}
	}

	return ""
}

// extractRemoteURI 提取远程URI
func extractRemoteURI(msg types.SipMessage) string {
	// 基于连接方向判断远程URI
	headers := msg.Headers()
	conn := msg.Connection()

	if conn == nil {
		// 无连接信息时，回退到传统判断
		if msg.IsRequest() {
			if to, exists := headers["To"]; exists {
				return parseURIFromHeader(to)
			}
		} else {
			if from, exists := headers["From"]; exists {
				return parseURIFromHeader(from)
			}
		}
		return ""
	}

	// 基于连接方向判断远程URI
	if conn.Direction == types.DirectionInbound {
		// 入栈消息：From头部代表远程端
		if from, exists := headers["From"]; exists {
			return parseURIFromHeader(from)
		}
	} else {
		// 出栈消息：To头部代表远程端
		if to, exists := headers["To"]; exists {
			return parseURIFromHeader(to)
		}
	}

	return ""
}

// extractLocalSeq 提取本地序列号
func extractLocalSeq(msg types.SipMessage) uint32 {
	// 在Proxy模式下，序列号的判断更复杂
	// 简化处理：如果是我们发出的消息，才算本地序列号
	conn := msg.Connection()
	if conn != nil && conn.Direction == types.DirectionOutbound {
		// 出栈消息，可能是我们生成的
		headers := msg.Headers()
		if cseq, exists := headers["CSeq"]; exists {
			return parseSeqFromCSeq(cseq)
		}
	}
	return 0
}

// extractRemoteSeq 提取远程序列号
func extractRemoteSeq(msg types.SipMessage) uint32 {
	// 只有从远程收到的请求才包含远程序列号
	if !msg.IsRequest() {
		return 0
	}

	conn := msg.Connection()
	if conn != nil && conn.Direction == types.DirectionInbound {
		// 入栈请求，来自远程端
		headers := msg.Headers()
		if cseq, exists := headers["CSeq"]; exists {
			return parseSeqFromCSeq(cseq)
		}
	}
	return 0
}

// extractBranchID 提取分支ID
func extractBranchID(msg types.SipMessage) string {
	// 从Via头中提取branch参数
	headers := msg.Headers()
	if via, exists := headers["Via"]; exists {
		return parseBranchFromVia(via)
	}
	return ""
}

// parseTagFromHeader 解析头部中的tag参数
func parseTagFromHeader(header string) string {
	// 解析From/To头部中的tag参数
	// 格式: "Display Name" <sip:user@domain>;tag=value
	tagStart := strings.Index(header, "tag=")
	if tagStart == -1 {
		return ""
	}

	tagStart += 4 // 跳过"tag="
	tagEnd := strings.IndexAny(header[tagStart:], ";,")
	if tagEnd == -1 {
		return strings.TrimSpace(header[tagStart:])
	}

	return strings.TrimSpace(header[tagStart : tagStart+tagEnd])
}

// parseURIFromHeader 解析头部中的URI
func parseURIFromHeader(header string) string {
	// 解析From/To头部中的URI
	// 格式: "Display Name" <sip:user@domain>;tag=value
	uriStart := strings.Index(header, "<")
	uriEnd := strings.Index(header, ">")

	if uriStart != -1 && uriEnd != -1 && uriEnd > uriStart {
		return header[uriStart+1 : uriEnd]
	}

	// 如果没有尖括号，可能是简单格式
	parts := strings.Fields(header)
	if len(parts) > 0 {
		uri := parts[0]
		if strings.HasPrefix(uri, "sip:") || strings.HasPrefix(uri, "sips:") {
			return uri
		}
	}

	return ""
}

// parseSeqFromCSeq 解析CSeq中的序列号
func parseSeqFromCSeq(cseq string) uint32 {
	// 解析CSeq头部中的序列号
	// 格式: "number method"，例如 "1 INVITE"
	parts := strings.Fields(strings.TrimSpace(cseq))
	if len(parts) >= 1 {
		if seq, err := strconv.ParseUint(parts[0], 10, 32); err == nil {
			return uint32(seq)
		}
	}
	return 0
}

// parseBranchFromVia 解析Via中的branch参数
func parseBranchFromVia(via string) string {
	// 解析Via头部中的branch参数
	// 格式: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK-xxx
	branchStart := strings.Index(via, "branch=")
	if branchStart == -1 {
		return ""
	}

	branchStart += 7 // 跳过"branch="
	branchEnd := strings.IndexAny(via[branchStart:], ";,")
	if branchEnd == -1 {
		return strings.TrimSpace(via[branchStart:])
	}

	return strings.TrimSpace(via[branchStart : branchStart+branchEnd])
}

// isRemoteRequest 判断是否为远程请求
func isRemoteRequest(msg types.SipMessage) bool {
	if !msg.IsRequest() {
		return false
	}

	conn := msg.Connection()
	if conn == nil {
		return true // 无连接信息时保守认为是远程请求
	}

	// 简化判断：入栈请求就是远程请求
	return conn.Direction == types.DirectionInbound
}
