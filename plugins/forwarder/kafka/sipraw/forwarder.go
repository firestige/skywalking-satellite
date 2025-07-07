package sipraw

import (
	"fmt"
	"reflect"

	"github.com/Shopify/sarama"
	"google.golang.org/grpc"
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"

	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/satellite/event"
)

const (
	// Name is the name of the SIP raw forwarder.
	Name     = "sip-raw-kafka-forwarder"
	ShowName = "SIP Raw Kafka Forwarder"
)

type Forwarder struct {
	config.CommonFields
	Topic    string `mapstructure:"topic"` // The forwarder topic.
	producer sarama.SyncProducer
}

func (f *Forwarder) Name() string {
	return Name
}

func (f *Forwarder) ShowName() string {
	return ShowName
}

func (f *Forwarder) Description() string {
	return "This is a synchronization Kafka forwarder for SIP raw data."
}

func (f *Forwarder) DefaultConfig() string {
	return `
# The remote topic.
topic: "sip-raw-topic"
`
}

func (f *Forwarder) Prepare(connection interface{}) error {
	client, ok := connection.(sarama.Client)
	if !ok {
		return fmt.Errorf("the %s is only accept the kafka client, but receive a %s",
			f.Name(), reflect.TypeOf(connection).String())
	}
	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		return err
	}
	f.producer = producer
	return nil
}

func (f *Forwarder) Forward(batch event.BatchEvents) error {
	return nil
}

func (f *Forwarder) ForwardType() v1.SniffType {
	return v1.SniffType_Logging
}

func (f *Forwarder) SyncForward(_ *v1.SniffData) (*v1.SniffData, grpc.ClientStream, error) {
	return nil, nil, fmt.Errorf("unsupport sync forward")
}

func (f *Forwarder) SupportedSyncInvoke() bool {
	return false
}
