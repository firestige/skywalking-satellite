package config

import (
	"sync"
)

var Cfg Config

var WgExitGroup sync.WaitGroup

type Config struct {
	Iface *InterfacesConfig `mapstructure:"iface"`
	// Logging            *logp.Logging
	Mode          string `mapstructure:"mode"`
	Dedup         bool   `mapstructure:"dedup"`
	Filter        string `mapstructure:"filter"`
	Discard       string `mapstructure:"discard"`
	DiscardMethod string `mapstructure:"discard_method"`
	DiscardIP     string `mapstructure:"discard_ip"`
	DiscardSrcIP  string `mapstructure:"discard_src_ip"`
	DiscardDstIP  string `mapstructure:"discard_dst_ip"`
	HepServer     string `mapstructure:"hep_server"`
	HepNodeName   string `mapstructure:"hep_node_name"`
	Reassembly    bool   `mapstructure:"reassembly"`
	SipAssembly   bool   `mapstructure:"sip_assembly"`
}

type InterfacesConfig struct {
	Device       string `mapstructure:"device"`
	Type         string `mapstructure:"type"`
	PortRange    string `mapstructure:"port_range"`
	Snaplen      int    `mapstructure:"snaplen"`
	BufferSizeMb int    `mapstructure:"buffer_size_mb"`
	EOFExit      bool   `mapstructure:"eof_exit"`
	FanoutID     uint   `mapstructure:"fanout_id"`
}
