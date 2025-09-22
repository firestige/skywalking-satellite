package sniffdata

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

type SipSequenceData struct {
	RawMsg    string `json:"raw_msg"`
	IsRequest bool   `json:"is_request"`
	From      string `json:"from"`
	To        string `json:"to"`
	Local     string `json:"local"`
	Timestamp int64  `json:"timestamp"`
	RefID     string `json:"ref_id"` // 对话ID或事务ID
	IsError   bool   `json:"is_error,omitempty"`
}

func NewSipSequenceData(msg types.SipMessage) *SipSequenceData {
	data := &SipSequenceData{
		RawMsg:    msg.String(),
		IsRequest: msg.IsRequest(),
		From:      msg.SrcURI(),   // ip 可能要通过dns 解析获得
		To:        msg.DstURI(),   // ip
		Local:     msg.LocalURI(), // 本地地址"ip:port"
		Timestamp: msg.CreatedAt(),
		RefID:     fmt.Sprintf("%s/%s", msg.CallID(), strings.ReplaceAll(msg.CSeq(), " ", "_")),
	}
	if resp, ok := msg.(types.SipResponse); ok {
		if resp.Status() >= 400 {
			data.IsError = true
		}
	}
	return data
}

func (data *SipSequenceData) String() string {
	// 转 json
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("SipSequenceData{error: %v}", err)
	}
	return string(jsonBytes)
}
