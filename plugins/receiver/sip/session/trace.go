package session

import (
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/utils"
)

type TraceProducer struct {
	serviceName        string
	instanceId         string
	segmentIdGenerator *utils.SegmentIDGenerator
}

func NewTraceProducer(serviceName, instanceId string) *TraceProducer {
	return &TraceProducer{
		serviceName:        serviceName,
		instanceId:         instanceId,
		segmentIdGenerator: utils.NewSegmentIDGenerator(instanceId),
	}
}
