//go:build linux
// +build linux

package sniffer

import (
	"fmt"
	"time"

	"golang.org/x/net/bpf"

	"github.com/google/gopacket"
	"github.com/google/gopacket/afpacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

type afpacketHandle struct {
	TPacket *afpacket.TPacket
}

func newAfpacketHandle(device string, snaplen int, blockSize int, numBlocks int,
	timeout time.Duration) (*afpacketHandle, error) {

	h := &afpacketHandle{}
	var err error
	h.TPacket, err = afpacket.NewTPacket(
		afpacket.OptInterface(device),
		afpacket.OptFrameSize(snaplen),
		afpacket.OptBlockSize(blockSize),
		afpacket.OptNumBlocks(numBlocks),
		afpacket.OptPollTimeout(timeout),
		afpacket.SocketRaw,
		afpacket.TPacketVersion3)

	return h, err
}

func (h *afpacketHandle) ReadPacketData() (data []byte, ci gopacket.CaptureInfo, err error) {
	return h.TPacket.ReadPacketData()
}

func (h *afpacketHandle) ZeroCopyReadPacketData() (data []byte, ci gopacket.CaptureInfo, err error) {
	return h.TPacket.ZeroCopyReadPacketData()
}

func (h *afpacketHandle) SetFanout(id uint16) error {
	return h.TPacket.SetFanout(afpacket.FanoutHashWithDefrag, id)
}

func (h *afpacketHandle) SetBPFFilter(filter string, snaplen int) error {
	// use pcap bpf compiler to get raw bpf instruction
	pcapBPF, err := pcap.CompileBPFFilter(h.LinkType(), snaplen, filter)
	if err != nil {
		return err
	}
	rawBPF := make([]bpf.RawInstruction, len(pcapBPF))
	for i, ri := range pcapBPF {
		rawBPF[i] = bpf.RawInstruction{Op: ri.Code, Jt: ri.Jt, Jf: ri.Jf, K: ri.K}
	}
	return h.TPacket.SetBPF(rawBPF)
}

func (h *afpacketHandle) LinkType() layers.LinkType {
	return layers.LinkTypeEthernet
}

func (h *afpacketHandle) Close() {
	h.TPacket.Close()
}

func (h *afpacketHandle) Stats() (uint, uint, error) {
	_, v3, err := h.TPacket.SocketStats()
	return v3.Packets(), v3.Drops(), err
}

func (h *afpacketHandle) IsErrTimeout(err error) bool {
	return err == afpacket.ErrTimeout
}

func afpacketComputeSize(targetSizeMb int, snaplen int, pageSize int) (
	frameSize int, blockSize int, numBlocks int, err error) {

	if snaplen < pageSize {
		frameSize = pageSize / (pageSize / snaplen)
	} else {
		frameSize = (snaplen/pageSize + 1) * pageSize
	}

	// 128 is the default from the gopacket library so just use that
	blockSize = frameSize * 128
	numBlocks = (targetSizeMb * 1024 * 1024) / blockSize

	if numBlocks == 0 {
		return 0, 0, 0, fmt.Errorf("buffer size too small")
	}

	return frameSize, blockSize, numBlocks, nil
}
