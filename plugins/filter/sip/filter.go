package sip

import (
	"fmt"

	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"

	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/satellite/event"
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

func (f *Filter) Process(ctx *event.OutputEventContext) {
	data, err := ctx.Get("http-packet-trace")
	if err != nil {
		fmt.Printf("Error getting data from context: %v\n", err)
		return
	}
	if data == nil {
		fmt.Println("No data found in context")
		return
	}
	segment := data.Data.(*v1.SniffData_Segment)
	fmt.Printf("Processing SIP data: %s\n", segment.Segment)
}
