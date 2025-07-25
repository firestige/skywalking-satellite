package sip

import (
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
)

func (r *Receiver) processUDPFrame(frame *types.RawFrameData) error {
	// 解析SIP消息
	sipMessage, err := r.sipParser.Parse(frame.Data)
	if err != nil {
		log.Logger.Debug("failed to parse SIP message:", err)
		// bad packet, ignore and continue, need statistics
		return nil
	}

	return r.processSipMessage(sipMessage)
}
