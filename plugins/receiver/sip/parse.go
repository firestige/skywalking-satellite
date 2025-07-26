package sip

import (
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/ghettovoice/gosip/sip"
	"github.com/ghettovoice/gosip/sip/parser"
)

type SipParser struct{}

func (p *SipParser) Parse(data []byte) (sip.Message, error) {
	parser := parser.NewPacketParser(&LoggerAdapter{log.Logger}) // Assuming a logger is not needed for this example
	msg, err := parser.ParseMessage(data)
	if err != nil {
		return nil, err
	}
	return msg, nil
}
