package sip

import (
	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/satellite/event"
)

const (
	// Name is the name of the SIP parser.
	Name     = "sip-parser"
	ShowName = "SIP Parser"
)

type Parser struct {
	config.CommonFields
}

func (p *Parser) Name() string {
	return Name
}

func (p *Parser) ShowName() string {
	return ShowName
}

func (p *Parser) Description() string {
	return "SIP Parser processes SIP protocol data, extracting and transforming relevant information for further analysis."
}

func (p *Parser) DefaultConfig() string {
	return ``
}

func (p *Parser) ParseBytes(bytes []byte) (event.BatchEvents, error) {
	// Implement the logic to parse bytes into events
	// For now, just return an empty batch
	return event.BatchEvents{}, nil
}

func (p *Parser) ParseStr(str string) (event.BatchEvents, error) {
	// Implement the logic to parse string into events
	// For now, just return an empty batch
	return event.BatchEvents{}, nil
}
