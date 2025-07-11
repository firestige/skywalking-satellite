package afpacket

import (
	"sync"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
)

type Event struct {
	Name      string
	Timestamp int64
	Payload   []byte
}

type EventLoop struct {
	events   chan *Event
	handlers []func(*Event) error
	wg       sync.WaitGroup
	stopChan chan struct{}
}

func NewEventLoop(bufferSize int) *EventLoop {
	return &EventLoop{
		events:   make(chan *Event, bufferSize),
		stopChan: make(chan struct{}),
		handlers: make([]func(*Event) error, 0),
	}
}

func (el *EventLoop) RegisterHandler(handler func(*Event) error) {
	el.handlers = append(el.handlers, handler)
}

func (el *EventLoop) Start() {
	el.wg.Add(1)
	go func() {
		defer el.wg.Done()
		for {
			select {
			case <-el.stopChan:
				return
			case event, ok := <-el.events:
				if !ok {
					return // Channel closed
				}
				el.processEvent(event)
			}
		}
	}()
}

func (el *EventLoop) processEvent(event *Event) {
	for _, handler := range el.handlers {
		if err := handler(event); err != nil {
			log.Logger.Fatal("Error processing event:", err)
		}
	}
}

func (el *EventLoop) Publish(event *Event) bool {
	select {
	case el.events <- event:
		return true
	default:
		return false // Channel is full, event not published
	}
}

func (el *EventLoop) Stop() {
	close(el.stopChan)
	el.wg.Wait()
	close(el.events)
}
