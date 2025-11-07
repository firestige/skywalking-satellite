package publish

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"unicode"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
)

type HEPConn struct {
	conn   net.Conn
	writer *bufio.Writer
}
type HEPOutputer struct {
	hepQueue chan []byte
	addr     []string
	client   []HEPConn
}

func writeAndFlush(client *HEPConn, data []byte, action string) (int, error) {
	hl, err := client.conn.Write(data)
	if err != nil {
		return 0, fmt.Errorf("error writing to socket during %s: %w", action, err)
	}

	if err := client.writer.Flush(); err != nil {
		return 0, fmt.Errorf("error flushing writer during %s: %w", action, err)
	}

	return hl, nil
}

func NewHEPOutputer(serverAddr string) (*HEPOutputer, error) {
	a := strings.Split(cutSpace(serverAddr), ",")
	l := len(a)
	h := &HEPOutputer{
		addr:     a,
		client:   make([]HEPConn, l),
		hepQueue: make(chan []byte, 20000),
	}
	errCnt := 0
	for n := range a {
		if err := h.ConnectServer(n); err != nil {
			log.Logger.Errorf("Error connecting to HEP server (%s): %v", h.addr[n], err)
			errCnt++
		}
	}
	if errCnt == l {
		return nil, fmt.Errorf("cannot establish a connection")
	}

	go h.Start()
	return h, nil
}

func (h *HEPOutputer) Close(n int) {
	if err := h.client[n].conn.Close(); err != nil {
		log.Logger.Errorf("Cannot close connection to %s: %v", h.addr[n], err)
	}
}

func (h *HEPOutputer) ReConnect(n int) (err error) {
	if err = h.ConnectServer(n); err != nil {
		log.Logger.Errorf("Error reconnecting to HEP server (%s): %v", h.addr[n], err)
		return err
	}
	h.client[n].writer.Reset(h.client[n].conn)

	return err
}

func (h *HEPOutputer) ConnectServer(n int) (err error) {

	if h.client[n].conn, err = net.Dial("udp", h.addr[n]); err != nil {
		return err
	}

	h.client[n].writer = bufio.NewWriterSize(h.client[n].conn, 8192)

	return err
}

func (h *HEPOutputer) Output(msg []byte) {
	h.hepQueue <- msg
}

func (h *HEPOutputer) Send(msg []byte) {
	for n := range h.addr {

		var err error

		if h.client[n].conn == nil || h.client[n].writer == nil {
			log.Logger.WithField("source", "collector").Debugf("Connection is not up， index: %d, Len: %d", n, len(h.addr))
			err = h.ReConnect(n)
			if err != nil {
				log.Logger.Errorf("ReConnect error: %v", err)
				continue
			} else {
				_, err = writeAndFlush(&h.client[n], msg, "sending message")
			}
		} else {
			_, err = writeAndFlush(&h.client[n], msg, "sending message")
			if err != nil {
				log.Logger.Errorf("Failed to send message: %s", err.Error())
			}
		}
	}
}

func (h *HEPOutputer) Start() {
	for msg := range h.hepQueue {
		h.Send(msg)
	}
}

func cutSpace(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, str)
}
