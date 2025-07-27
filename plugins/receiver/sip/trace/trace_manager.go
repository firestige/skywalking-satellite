package sip

import (
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/sniffdata"
	"github.com/apache/skywalking-satellite/plugins/receiver/sip/types"
)

type TraceContext struct {
	callIDs []string
	builder *sniffdata.SegmentBuilder
}

type TraceManager struct {
	serviceName       string
	serviceInstanceId string
	traceContext      map[string]*TraceContext // key: trace ID
	mappings          map[string]string        // key: call-id, value: trace ID
}

func NewTraceManager(serviceName, serviceInstanceId string) *TraceManager {
	return &TraceManager{
		serviceName:       serviceName,
		serviceInstanceId: serviceInstanceId,
		traceContext:      make(map[string]*TraceContext),
		mappings:          make(map[string]string),
	}
}

func (m *TraceManager) GetTraceContext(traceID string) *TraceContext {
	if ctx, exists := m.traceContext[traceID]; exists {
		return ctx
	}
	return nil
}

func (m *TraceManager) CreateTraceContext(traceID string) *TraceContext {
	if _, exists := m.traceContext[traceID]; !exists {
		m.traceContext[traceID] = &TraceContext{
			callIDs: make([]string, 0),
			builder: sniffdata.NewSegmentBuilder(m.serviceName, m.serviceInstanceId),
		}
	}
	return m.traceContext[traceID]
}

func (m *TraceManager) AddDialogToTrace(dialog types.Dialog) {
	callID := dialog.CallID()
	traceID := m.mappings[callID]
	ctx := m.GetTraceContext(traceID)
	builder := sniffdata.NewSegmentBuilder(m.serviceName, m.serviceInstanceId)
	ctx.builder = builder
}
