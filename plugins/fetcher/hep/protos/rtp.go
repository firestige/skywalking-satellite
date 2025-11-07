package protos

import (
	"github.com/apache/skywalking-satellite/plugins/fetcher/hep/layers"
	"github.com/google/gopacket"
)

func NewRTP(raw []byte) string {
	rtpl := gopacket.NewPacket(raw, layers.LayerTypeRTP, gopacket.DecodeOptions{Lazy: true, NoCopy: true})
	rtp, ok := rtpl.Layers()[0].(*layers.RTP)
	if !ok {
		//return nil
		return "this is not a RTP packet!"
	}

	return rtp.String()
}
