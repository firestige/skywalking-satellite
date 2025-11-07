package publish

import (
	"sync/atomic"
	"time"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/fetcher/hep/config"
	"github.com/apache/skywalking-satellite/plugins/fetcher/hep/decoder"
)

type Outputer interface {
	Output(msg []byte)
}

type Publisher struct {
	pubCount uint64
	outputer Outputer
}

func NewPublisher(out Outputer) *Publisher {
	p := &Publisher{
		outputer: out,
		pubCount: 0,
	}

	go p.Start(decoder.PacketQueue)
	go p.printStats()
	return p
}

func (pub *Publisher) output(msg []byte) {
	defer func() {
		if err := recover(); err != nil {
			log.Logger.Errorf("recover %v", err)
		}
	}()
	pub.outputer.Output(msg)
}

func (pub *Publisher) Start(pq chan *decoder.Packet) {
	for pkt := range pq {

		atomic.AddUint64(&pub.pubCount, 1)

		if pkt.Version == 255 {
			//this is EXIT
			log.Logger.Info("received exit signal")
			if config.Cfg.Iface.EOFExit {
				log.Logger.Info("exiting...")
				config.WgExitGroup.Done()
				return
			}
			break
		} else {

			msg, err := EncodeHEP(pkt)
			if err != nil {
				log.Logger.Warnf("%v", err)
				continue
			}

			pub.output(msg)
		}
	}
}

func (pub *Publisher) printStats() {
	for {
		<-time.After(1 * time.Minute)
		go func() {
			log.Logger.Infof("Packets since last minute sent: %d", atomic.LoadUint64(&pub.pubCount))
			atomic.StoreUint64(&pub.pubCount, 0)
		}()
	}
}
