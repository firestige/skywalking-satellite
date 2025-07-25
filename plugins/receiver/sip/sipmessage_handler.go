package sip

import (
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/sniffdata"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

func (r *Receiver) processSipMessage(msg types.SipMessage) error {
	session, err := r.sessionManager.GetOrCreateSession(msg)
	if err != nil {
		log.Logger.Error("failed to get or create session:", err)
		return err
	}

	if err != nil {
		log.Logger.Error("failed to get or create session:", err)
		return err
	}

	traceData := sniffdata.NewSegmentBuilder().Build()
	r.OutputChannel <- traceData
	logData := sniffdata.NewLogBuilder().Build()
	r.OutputChannel <- logData
	return nil
}
