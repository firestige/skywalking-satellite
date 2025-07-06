package sip

import (
	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/satellite/event"
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"
)

const (
	// Name is the name of the SIP filter.
	Name     = "sip-filter"
	ShowName = "SIP Filter"
)

type Filter struct {
	config.CommonFields
}

func (f *Filter) Name() string {
	return Name
}

func (f *Filter) ShowName() string {
	return ShowName
}

func (f *Filter) Description() string {
	return "SIP Filter processes SIP protocol data, extracting and transforming relevant information for further analysis."
}

func (f *Filter) DefaultConfig() string {
	return ``
}

func (f *Filter) processes(ctx *event.OutputEventContext) {
	sipRaw, ok := ctx.Get("sipraw")
	if !ok {
		ctx.Error("SIP raw data not found in context")
		return
	}
	sipData, ok := sipRaw.(*v1.SipRaw)
	if !ok {
		ctx.Error("Invalid SIP raw data type")
		return
	}
	// Process the SIP data as needed
	// todo 解析数据

	// 生成并放入trace数据
	trace := &v1.SniffData{
		Name: "sip-trace",
		Data: &v1.SniffData_Segment{},
	}
	ctx.Put(trace)
	// 生成并放入关联log数据（实际上是sip报文的索引）
	ctx.Put(&v1.SniffData{
		Name: "sip-log",
		Data: &v1.SniffData_Logging{
			Logging: &v1.Logging{
				TraceId:   sipData.TraceId,
				SpanId:    sipData.SpanId,
				Timestamp: sipData.Timestamp,
				Level:     v1.Logging_INFO,
				Message:   "Processed SIP data",
			},
		},
	})
	// 生成并放入原始SIP数据
	ctx.Put(&v1.SniffData{
		Name: "sipraw",
		Data: &v1.SniffData_SipRaw{
			SipRaw: sipData,
		},
	})
}
