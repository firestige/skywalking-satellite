package sniffer

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/fetcher/hep/config"
	"github.com/apache/skywalking-satellite/plugins/fetcher/hep/decoder"
	"github.com/apache/skywalking-satellite/plugins/fetcher/hep/publish"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

type SnifferSetup struct {
	afpacketHandle *afpacketHandle
	config         *config.InterfacesConfig
	isAlive        bool
	mode           string
	bpf            string
	filter         []string
	discard        []string
	worker         Worker
	DataSource     gopacket.PacketDataSource
}

type MainWorker struct {
	publisher *publish.Publisher
	decoder   *decoder.Decoder
}

type Worker interface {
	OnPacket(data []byte, ci *gopacket.CaptureInfo)
}

type WorkerFactory func(layers.LinkType) (Worker, error)

func NewWorker(lt layers.LinkType) (Worker, error) {
	var o publish.Outputer
	var err error

	o, err = publish.NewHEPOutputer(config.Cfg.HepServer)
	if err != nil {
		return nil, err
	}

	p := publish.NewPublisher(o)
	d := decoder.NewDecoder(lt)
	w := &MainWorker{publisher: p, decoder: d}
	return w, nil
}

func (mw *MainWorker) OnPacket(data []byte, ci *gopacket.CaptureInfo) {
	mw.decoder.Process(data, ci)
}

func (sniffer *SnifferSetup) setFromConfig() error {
	// var err error

	if sniffer.config.Snaplen <= 0 {
		sniffer.config.Snaplen = 65535
	}

	switch sniffer.mode {
	case "SIP":
		sniffer.bpf = "(tcp or sctp) and greater 42 and portrange " + sniffer.config.PortRange + " or (udp and greater 128 and portrange " + sniffer.config.PortRange + " or ip[6:2] & 0x1fff != 0 or ip6[6]=44)"
	case "SIPDNS":
		sniffer.bpf = "(tcp or sctp) and greater 42 and portrange " + sniffer.config.PortRange + " or (udp and greater 128 and portrange " + sniffer.config.PortRange + " or ip[6:2] & 0x1fff != 0 or ip6[6]=44) or (ip and ip[6] & 0x2 = 0 and ip[6:2] & 0x1fff = 0 and udp and udp[8] & 0xc0 = 0x80 and udp[9] >= 0xc8 && udp[9] <= 0xcc) or (greater 32 and ip and dst port 53)"
	case "SIPLOG":
		sniffer.bpf = "(tcp or sctp) and greater 42 and portrange " + sniffer.config.PortRange + " or (udp and greater 128 and portrange " + sniffer.config.PortRange + " or ip[6:2] & 0x1fff != 0 or ip6[6]=44) or (ip and ip[6] & 0x2 = 0 and ip[6:2] & 0x1fff = 0 and udp and udp[8] & 0xc0 = 0x80 and udp[9] >= 0xc8 && udp[9] <= 0xcc) or (greater 128 and (dst port 514 or port 2223))"
	case "SIPRTP":
		sniffer.bpf = "(tcp or sctp) and greater 42 and portrange " + sniffer.config.PortRange + " or (udp and greater 128 and portrange " + sniffer.config.PortRange + " or ip[6:2] & 0x1fff != 0 or ip6[6]=44) or (ip and ip[6] & 0x2 = 0 and ip[6:2] & 0x1fff = 0 and udp and udp[8] & 0xc0 = 0x80)"
	default:
		sniffer.mode = "SIPRTCP"
		sniffer.bpf = "(tcp or sctp) and greater 42 and portrange " + sniffer.config.PortRange + " or (udp and greater 128 and portrange " + sniffer.config.PortRange + " or ip[6:2] & 0x1fff != 0 or ip6[6]=44) or (ip and ip[6] & 0x2 = 0 and ip[6:2] & 0x1fff = 0 and udp and udp[8] & 0xc0 = 0x80 and udp[9] >= 0xc8 && udp[9] <= 0xcc)"
	}

	log.Logger.Infof("%#v", config.Cfg)
	log.Logger.Infof("%#v", config.Cfg.Iface)
	log.Logger.Infof("bpf: %s", sniffer.bpf)
	if len(sniffer.discard) > 0 {
		log.Logger.Infof("discard: %#v", sniffer.discard)
	}
	if len(sniffer.filter) > 0 {
		log.Logger.Infof("filter: %#v", sniffer.filter)
	}
	log.Logger.Infof("ostype: %s, osarch: %s", runtime.GOOS, runtime.GOARCH)

	switch sniffer.config.Type {
	case "af_packet":
		if sniffer.config.BufferSizeMb <= 0 {
			sniffer.config.BufferSizeMb = 32
		}

		szFrame, szBlock, numBlocks, err := afpacketComputeSize(sniffer.config.BufferSizeMb, sniffer.config.Snaplen, os.Getpagesize())
		if err != nil {
			return fmt.Errorf("setting af_packet computesize: %v", err)
		}

		sniffer.afpacketHandle, err = newAfpacketHandle(sniffer.config.Device, szFrame, szBlock, numBlocks, 1*time.Second)
		if err != nil {
			return fmt.Errorf("setting af_packet handle: %v", err)
		}

		if sniffer.config.FanoutID > 0 {
			err = sniffer.afpacketHandle.SetFanout(uint16(sniffer.config.FanoutID))
			if err != nil {
				return fmt.Errorf("SetFanout '%d' for af_packet: %v", uint16(sniffer.config.FanoutID), err)
			}
		}

		err = sniffer.afpacketHandle.SetBPFFilter(sniffer.bpf, sniffer.config.Snaplen)
		if err != nil {
			return fmt.Errorf("SetBPFFilter '%s' for af_packet: %v", sniffer.bpf, err)
		}

		sniffer.DataSource = gopacket.PacketDataSource(sniffer.afpacketHandle)

	default:
		return fmt.Errorf("unknown sniffer type: %s", sniffer.config.Type)
	}

	return nil
}

//mode string, cfg *config.InterfacesConfig, collector string, onlySip bool

func New(cfgMain *config.Config) (*SnifferSetup, error) {
	var err error
	sniffer := &SnifferSetup{}
	sniffer.config = cfgMain.Iface
	sniffer.mode = cfgMain.Mode

	if sniffer.config.Device == "any" && (runtime.GOOS == "windows" || runtime.GOOS == "darwin") {
		_, err := ListDeviceNames(true, false)
		return nil, fmt.Errorf("%v -i any is not supported on %s\nPlease use one of the above devices", err, runtime.GOOS)
	}

	if sniffer.config.Device == "" {
		_, err := ListDeviceNames(true, false)
		return nil, fmt.Errorf("%v Please use one of the above devices", err)
	}

	err = sniffer.setFromConfig()
	if err != nil {
		return nil, err
	}

	sniffer.worker, err = NewWorker(sniffer.Datalink())
	if err != nil {
		return nil, err
	}

	sniffer.isAlive = true

	go sniffer.printStats()

	return sniffer, nil
}

func (sniffer *SnifferSetup) Run() error {
	var (
		retError error
	)

LOOP:
	for sniffer.isAlive {

		data, ci, err := sniffer.DataSource.ReadPacketData()

		if err == pcap.NextErrorTimeoutExpired || sniffer.afpacketHandle.IsErrTimeout(err) || err == syscall.EINTR {
			continue
		}

		if err != nil {
			retError = fmt.Errorf("sniffing error: %s", err)
			sniffer.isAlive = false
			continue
		}

		if len(data) == 0 {
			continue
		}

		if len(sniffer.filter) > 0 {
			for i := range sniffer.filter {
				if !bytes.Contains(data, []byte(sniffer.filter[i])) {
					continue LOOP
				}
			}
		}
		if len(sniffer.discard) > 0 {
			for i := range sniffer.discard {
				if bytes.Contains(data, []byte(sniffer.discard[i])) {
					continue LOOP
				}
			}
		}

		sniffer.worker.OnPacket(data, &ci)
	}
	sniffer.Close()
	return retError
}

func (sniffer *SnifferSetup) Close() error {
	sniffer.afpacketHandle.Close()
	return nil
}

func (sniffer *SnifferSetup) Datalink() layers.LinkType {
	return sniffer.afpacketHandle.LinkType()
}

func (sniffer *SnifferSetup) printStats() {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	ticker := time.NewTicker(1 * time.Minute)

	for {
		select {
		case <-ticker.C:
			p, d, err := sniffer.afpacketHandle.Stats()
			if err != nil {
				log.Logger.Warnf("Stats err: %v", err)
			}
			log.Logger.Infof("Stats {received dropped}: {%d %d}", p, d)

		case <-signals:
			log.Logger.Info("Sniffer received stop signal")
			time.Sleep(500 * time.Millisecond)
			os.Exit(0)
		}
	}
}
