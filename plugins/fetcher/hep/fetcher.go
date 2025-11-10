package hep

import (
	"context"
	"encoding/json"

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
	type: afpacket
	rotation_time: 60
	port_range: ""
	snaplen: 65535
	buffer_size_mb: 1024
	eof_exit: false
	fanout_id: 1
  mode: "SIPRTP"
  dedup: false
  filter: ""
  discard: ""
  discard_method: "blacklist"
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
	// 输出调用前的 HepConfig
	{
		b, err := json.Marshal(f.HepConfig)
		if err != nil {
			log.Logger.Errorf("failed to marshal HepConfig: %v", err)
		} else {
			log.Logger.Infof("Prepare: HepConfig before apply: %s", b)
		}
	}

	applyConfig(f.HepConfig)

	// 输出调用后的 hepconfig.Cfg
	{
		b, err := json.Marshal(hepconfig.Get())
		if err != nil {
			log.Logger.Errorf("failed to marshal hepconfig.Cfg: %v", err)
		} else {
			log.Logger.Infof("Prepare: hepconfig.Cfg after apply: %s", b)
		}
	}

	f.channel = make(chan *v1.SniffData, 100)

}

func applyConfig(src *hepconfig.Config) {
	// 构造新对象，原子更新
	newCfg := &hepconfig.Config{
		Iface: &hepconfig.InterfacesConfig{
			Device:       src.Iface.Device,
			Type:         src.Iface.Type,
			PortRange:    src.Iface.PortRange,
			Snaplen:      src.Iface.Snaplen,
			BufferSizeMb: src.Iface.BufferSizeMb,
			EOFExit:      src.Iface.EOFExit,
			FanoutID:     src.Iface.FanoutID,
		},
		Mode:          src.Mode,
		Dedup:         src.Dedup,
		Filter:        src.Filter,
		Discard:       src.Discard,
		DiscardMethod: src.DiscardMethod,
		DiscardIP:     src.DiscardIP,
		DiscardSrcIP:  src.DiscardSrcIP,
		DiscardDstIP:  src.DiscardDstIP,
		HepServer:     src.HepServer,
		HepNodeName:   src.HepNodeName,
		Reassembly:    src.Reassembly,
		SipAssembly:   src.SipAssembly,
	}
	// 原子存储
	hepconfig.Store(newCfg)
}

func (f *Fetcher) Fetch(ctx context.Context) {
	var err error
	f.captture, err = sniffer.New(f.HepConfig)
	if err != nil {
		log.Logger.Errorf("failed to create sniffer: %v", err)
		return
	}
	if f.captture == nil {
		return
		log.Logger.Errorf("sniffer is nil, cannot run fetcher")
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
