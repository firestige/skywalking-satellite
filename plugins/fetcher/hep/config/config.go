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
	HepNodePW     string `mapstructure:"hep_node_pw"`
	HepNodeID     uint   `mapstructure:"hep_node_id"`
	HepNodeName   string `mapstructure:"hep_node_name"`
	Network       string `mapstructure:"network"`
	Reassembly    bool   `mapstructure:"reassembly"`
	SipAssembly   bool   `mapstructure:"sip_assembly"`
	// 以下是publish config 可能用不上
	SendRetries        uint   `mapstructure:"send_retries"`
	KeepAlive          uint   `mapstructure:"keep_alive"`
	Version            bool   `mapstructure:"version"`
	SkipVerify         bool   `mapstructure:"skip_verify"`
	HEPBufferDebug     bool   `mapstructure:"hep_buffer_debug"`
	HEPBufferEnable    bool   `mapstructure:"hep_buffer_enable"`
	HEPBufferSize      string `mapstructure:"hep_buffer_size"`
	HEPBufferFile      string `mapstructure:"hep_buffer_file"`
	MaxBufferSizeBytes int64  `mapstructure:"max_buffer_size_bytes"`
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
