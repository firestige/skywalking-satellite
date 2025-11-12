package protos

import (
	"fmt"

	"github.com/apache/skywalking-satellite/plugins/fetcher/hep/layers"
	"github.com/google/gopacket"
)

func NewRTP(raw []byte) (*layers.RTP, error) {
	rtpl := gopacket.NewPacket(raw, layers.LayerTypeRTP, gopacket.DecodeOptions{Lazy: true, NoCopy: true})
	rtp, ok := rtpl.Layers()[0].(*layers.RTP)
	if !ok {
		//return nil
		return nil, fmt.Errorf("this is not a RTP packet!")
	}

	return rtp, nil
}
