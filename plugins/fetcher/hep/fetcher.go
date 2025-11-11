package hep

import (
	"context"

	"github.com/apache/skywalking-satellite/internal/pkg/log"

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
    type: af_packet
    rotation_time: 60
    port_range: "10000-50000"
    snaplen: 65535
    buffer_size_mb: 1024
    eof_exit: false
    fanout_id: 1
  mode: "SIPRTP"
  dedup: false
  filter: ""
  discard: ""
  discard_method: ""
  discard_ip: ""
  discard_src_ip: ""
  discard_dst_ip: ""
  hep_server: "10.244.12.232:9090"
  hep_node_name: "satellite_hep_node"
  reassembly: true
  sip_assembly: true
plugin_name: hep_fetcher
`
}

func (f *Fetcher) Prepare() {
	f.channel = make(chan *v1.SniffData, 100)
}

func (f *Fetcher) Fetch(ctx context.Context) {
	var err error
	f.captture, err = sniffer.New(f.HepConfig)
	if err != nil {
		log.Logger.Errorf("failed to create sniffer: %v", err)
		return
	}
	if f.captture == nil {
		log.Logger.Errorf("sniffer is nil, cannot run fetcher")
		return
	}
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
