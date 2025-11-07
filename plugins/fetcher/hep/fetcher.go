package hep

import (
	"context"

	"github.com/apache/skywalking-satellite/internal/pkg/config"
	hepconfig "github.com/apache/skywalking-satellite/plugins/fetcher/hep/config"
	"github.com/apache/skywalking-satellite/plugins/fetcher/hep/sniffer"
	forwarder "github.com/apache/skywalking-satellite/plugins/forwarder/api"
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"
)

const (
	Name        = "hep_fetcher"
	ShowName    = "HEP Fetcher"
	Description = "HEP Fetcher is used to fetch HEP protocol data."
)

type Fetcher struct {
	config.CommonFields
	HepConfig *hepconfig.Config `mapstructure:"hep_config" yaml:"hep_config,omitempty"`

	captture *sniffer.SnifferSetup
	channel  chan *v1.SniffData
}

func (f *Fetcher) Name() string {
	return Name
}

func (f *Fetcher) ShowName() string {
	return ShowName
}

func (f *Fetcher) Description() string {
	return Description
}

func (f *Fetcher) DefaultConfig() string {
	return `
hep_config:
  iface:
    device: eth0
	type: afpacket
	rotation_time: 60
	portRange: ""
	snaplen: 65535
	buffer_size_mb: 1024
	eof_exit: false
	fanout: 1
  mode: "SIPRTP"
  dedup: false
  filter: ""
  discard: ""
  discard_method: "blacklist"
  discard_ip: ""
  discard_src_ip: ""
  discard_dst_ip: ""
  hep_server: "<hep_server>"
  hep_node_pw: "mypassword"
  hep_node_id: 1234
  hep_node_name: "satellite_hep_node"
  network: "udp"
  reassembly: true
  sip_assembly: true
  send_retries: 3
  keep_alive: 30
  version: false
  skip_verify: false
  hep_buffer_debug: false
  hep_buffer_enable: false
  hep_buffer_size: "10MB"
  hep_buffer_file: "hep_buffer.dat"
  max_buffer_size_bytes: 1073741824 #1GB
plugin_name: hep_fetcher
`
}

func (f *Fetcher) Prepare() {
	f.channel = make(chan *v1.SniffData, 100)
	f.captture, _ = sniffer.New(f.HepConfig)
}

func (f *Fetcher) Fetch(ctx context.Context) {
	f.captture.Run()
}

func (f *Fetcher) Channel() <-chan *v1.SniffData {
	return f.channel
}

func (f *Fetcher) Shutdown(context.Context) error {
	return f.captture.Close()
}

func (f *Fetcher) SupportForwarders() []forwarder.Forwarder {
	return nil
}
