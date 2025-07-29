package sip

import (
	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/ghettovoice/gosip/sip"
	"github.com/ghettovoice/gosip/sip/parser"
	"github.com/sirupsen/logrus"
)

type SipParser struct {
	delegate *parser.PacketParser
}

func NewSipParser() *SipParser {
	return &SipParser{
		delegate: parser.NewPacketParser(&LoggerAdapter{log.Logger}),
	}
}

func (p *SipParser) Parse(data []byte) (sip.Message, error) {
	msg, err := p.delegate.ParseMessage(data)
	if err != nil {
		if log.Logger.IsLevelEnabled(logrus.DebugLevel) {
			log.Logger.WithError(err).Debugf("Failed to parse SIP message: %s", data)
		} else {
			log.Logger.WithError(err).Errorf("Failed to parse SIP message")
		}
		return nil, err
	}
	return msg, nil
}
