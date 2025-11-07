package decoder

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
)

func (d *Decoder) flushFragments(dt time.Duration) {
	ticker := time.NewTicker(dt)
	for range ticker.C {
		d.defrag4.DiscardOlderThan(time.Now().Add(-dt))
		d.defrag6.DiscardOlderThan(time.Now().Add(-dt))
	}
}

func (d *Decoder) flushTCPAssembler(dt time.Duration) {
	ticker := time.NewTicker(dt)
	for range ticker.C {
		d.asm.FlushOlderThan(time.Now().Add(-dt))
	}
}

// MarshalJSON implements json marshal functions for Packet
func (p *Packet) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Version   byte
		Protocol  byte
		SrcIP     net.IP
		DstIP     net.IP
		SrcPort   uint16
		DstPort   uint16
		Tsec      uint32
		Tmsec     uint32
		ProtoType byte
		Payload   string
		CID       string
		Vlan      uint16
	}{
		Version:   p.Version,
		Protocol:  p.Protocol,
		SrcIP:     p.SrcIP,
		DstIP:     p.DstIP,
		SrcPort:   p.SrcPort,
		DstPort:   p.DstPort,
		Tsec:      p.Tsec,
		Tmsec:     p.Tmsec,
		ProtoType: p.ProtoType,
		Payload:   string(p.Payload),
		CID:       string(p.CID),
		Vlan:      p.Vlan,
	})
}

func (d *Decoder) printPacketStats() {
	log.Logger.Infof("Packets since last minute IPv4: %d, IPv6: %d, UDP: %d, TCP: %d, SCTP: %d, RTCP: %d, RTCPFail: %d, DNS: %d, HEP: %d, duplicate: %d, fragments: %d, unknown: %d",
		atomic.LoadUint64(&d.ip4Count),
		atomic.LoadUint64(&d.ip6Count),
		atomic.LoadUint64(&d.udpCount),
		atomic.LoadUint64(&d.tcpCount),
		atomic.LoadUint64(&d.sctpCount),
		atomic.LoadUint64(&d.rtcpCount),
		atomic.LoadUint64(&d.rtcpFailCount),
		atomic.LoadUint64(&d.dnsCount),
		atomic.LoadUint64(&d.hepCount),
		atomic.LoadUint64(&d.dupCount),
		atomic.LoadUint64(&d.fragCount),
		atomic.LoadUint64(&d.unknownCount),
	)
	atomic.StoreUint64(&d.ip4Count, 0)
	atomic.StoreUint64(&d.ip6Count, 0)
	atomic.StoreUint64(&d.udpCount, 0)
	atomic.StoreUint64(&d.tcpCount, 0)
	atomic.StoreUint64(&d.sctpCount, 0)
	atomic.StoreUint64(&d.rtcpCount, 0)
	atomic.StoreUint64(&d.rtcpFailCount, 0)
	atomic.StoreUint64(&d.dnsCount, 0)
	atomic.StoreUint64(&d.hepCount, 0)
	atomic.StoreUint64(&d.dupCount, 0)
	atomic.StoreUint64(&d.fragCount, 0)
	atomic.StoreUint64(&d.unknownCount, 0)
}

func (d *Decoder) printStats(dt time.Duration) {
	ticker := time.NewTicker(dt)
	for range ticker.C {
		d.printPacketStats()
	}
}

// Extract header value from RFC2822 like header data.
// Does not allow whitespaces before colon, but allows them after.
// headerNames must be array of possible header names without colon.
// E.g. "Call-ID", "Call-Id", "call-id", "i".
func getHeaderValue(headerNames [][]byte, data []byte) ([]byte, error) {
	var startPos int = -1
	var headerName []byte
	var buffer [60]byte // use large enough buffer for header name and separators on stack for fast append
	var search []byte
	for hederNameIdx := range headerNames {
		headerName = headerNames[hederNameIdx]
		// Check if first header.
		if bytes.HasPrefix(data, headerName) {
			if len(data) > len(headerName) && data[len(headerName)] == ':' {
				startPos = 0
				break
			}
		}
		// Check if other header.
		search = append(append(append(buffer[:0], '\r', '\n'), headerName...), ':')
		startPos = bytes.Index(data, search)
		if startPos >= 0 {
			// Skip new line
			startPos += 2
			break
		}
	}
	if startPos < 0 {
		return nil, errors.New("no such header")
	}
	endPos := bytes.Index(data[startPos:], []byte("\r\n"))
	if endPos < 0 {
		return nil, errors.New("no such header")
	}
	return bytes.TrimSpace(data[startPos+len(headerName)+1 : startPos+endPos]), nil
}

// Header names for use with getHeaderValue,
var (
	contentTypeHeaderNames = [][]byte{
		[]byte("Content-Type"),
		[]byte("Content-type"),
		[]byte("content-type"),
		[]byte("CONTENT-TYPE"),
		[]byte("c"),
	}
	contentLengthHeaderNames = [][]byte{
		[]byte("Content-Length"),
		[]byte("Content-length"),
		[]byte("content-length"),
		[]byte("CONTENT-LENGTH"),
		[]byte("l"),
	}
	callIdHeaderNames = [][]byte{
		[]byte("Call-ID"),
		[]byte("Call-Id"),
		[]byte("Call-id"),
		[]byte("call-id"),
		[]byte("CALL-ID"),
		[]byte("i"),
	}
)
