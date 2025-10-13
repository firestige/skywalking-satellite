package sip

import (
	"testing"

	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
	"github.com/ghettovoice/gosip/sip/parser"
	"github.com/sirupsen/logrus"
)

func TestParseSipRequestWithMultipleViaHeaders(t *testing.T) {
	// 创建一个包含两个 Via 头的 SIP 请求消息
	// 注意：SIP 消息必须使用 CRLF (\r\n) 作为行结束符
	sipMessage := "INVITE sip:alice@example.com SIP/2.0\r\n" +
		"Via: SIP/2.0/UDP proxy.example.com:5060;branch=z9hG4bK123456\r\n" +
		"Via: SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK789012\r\n" +
		"Max-Forwards: 70\r\n" +
		"From: <sip:bob@example.com>;tag=12345\r\n" +
		"To: <sip:alice@example.com>\r\n" +
		"Call-ID: abc123@example.com\r\n" +
		"CSeq: 1 INVITE\r\n" +
		"Contact: <sip:bob@client.example.com>\r\n" +
		"Content-Type: application/sdp\r\n" +
		"Content-Length: 0\r\n" +
		"\r\n"

	// 创建 SIP 解析器，传入有效的 logger
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	packetParser := parser.NewPacketParser(&LoggerAdapter{logger: logger})

	// 直接使用 gosip parser 解析消息
	goSipMsg, err := packetParser.ParseMessage([]byte(sipMessage))
	if err != nil {
		t.Fatalf("Failed to parse SIP message: %v", err)
	}

	// 调试：打印原始消息
	t.Logf("Original SIP message:\n%s", sipMessage)

	// 调试：检查 gosip 解析的 Via 头
	vias := goSipMsg.GetHeaders("via")
	if len(vias) == 0 {
		t.Fatal("No Via headers found in gosip message")
	}
	t.Logf("Gosip parsed Via headers count: %d", len(vias))
	for i, via := range vias {
		t.Logf("Gosip Via[%d]: %s", i, via.String())
	}

	// 转换为内部类型
	conn := &types.Connection{
		SrcIp:   "192.168.1.100",
		SrcPort: 5060,
		DstIp:   "192.168.1.200",
		DstPort: 5060,
	}
	req := FromGoSip(goSipMsg, conn, 1234567890)

	// 验证 Via 头数量
	viaHeaders := req.Via()
	viaCount := len(viaHeaders)

	t.Logf("Parsed Via headers count: %d", viaCount)
	for i, via := range viaHeaders {
		t.Logf("Via[%d]: %s", i, via)
	}

	// 断言 Via 头数量不少于 2
	if viaCount < 2 {
		t.Errorf("Expected at least 2 Via headers, but got %d", viaCount)
	}

	// 验证具体的 Via 头内容
	if viaCount >= 1 {
		if !contains(viaHeaders[0], "proxy.example.com") {
			t.Errorf("First Via header should contain proxy.example.com, got: %s", viaHeaders[0])
		}
	}

	if viaCount >= 2 {
		if !contains(viaHeaders[1], "client.example.com") {
			t.Errorf("Second Via header should contain client.example.com, got: %s", viaHeaders[1])
		}
	}
}

// 辅助函数：检查字符串是否包含子串
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestParseSipRequestWithCommaSeparatedViaHeaders(t *testing.T) {
	// 创建一个包含逗号分隔 Via 头的 SIP 请求消息
	sipMessage := "INVITE sip:alice@example.com SIP/2.0\r\n" +
		"Via: SIP/2.0/UDP proxy.example.com:5060;branch=z9hG4bK123456, SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK789012\r\n" +
		"Max-Forwards: 70\r\n" +
		"From: <sip:bob@example.com>;tag=12345\r\n" +
		"To: <sip:alice@example.com>\r\n" +
		"Call-ID: abc123@example.com\r\n" +
		"CSeq: 1 INVITE\r\n" +
		"Contact: <sip:bob@client.example.com>\r\n" +
		"Content-Type: application/sdp\r\n" +
		"Content-Length: 0\r\n" +
		"\r\n"

	// 创建 SIP 解析器，传入有效的 logger
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	packetParser := parser.NewPacketParser(&LoggerAdapter{logger: logger})

	// 直接使用 gosip parser 解析消息
	goSipMsg, err := packetParser.ParseMessage([]byte(sipMessage))
	if err != nil {
		t.Fatalf("Failed to parse SIP message: %v", err)
	}

	// 调试：打印原始消息
	t.Logf("Original SIP message:\n%s", sipMessage)

	// 调试：检查 gosip 解析的 Via 头
	vias := goSipMsg.GetHeaders("via")
	if len(vias) == 0 {
		t.Fatal("No Via headers found in gosip message")
	}
	t.Logf("Gosip parsed Via headers count: %d", len(vias))
	for i, via := range vias {
		t.Logf("Gosip Via[%d]: %s", i, via.String())
	}

	// 转换为内部类型
	conn := &types.Connection{
		SrcIp:   "192.168.1.100",
		SrcPort: 5060,
		DstIp:   "192.168.1.200",
		DstPort: 5060,
	}
	req := FromGoSip(goSipMsg, conn, 1234567890)

	// 验证 Via 头数量
	viaHeaders := req.Via()
	viaCount := len(viaHeaders)

	t.Logf("Parsed Via headers count: %d", viaCount)
	for i, via := range viaHeaders {
		t.Logf("Via[%d]: %s", i, via)
	}

	// 断言 Via 头数量不少于 2
	if viaCount < 2 {
		t.Errorf("Expected at least 2 Via headers from comma-separated format, but got %d", viaCount)
	}
}
