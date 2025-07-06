package sipraw

import (
	"fmt"
	"reflect"

	"github.com/Shopify/sarama"
	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/satellite/event"
	"google.golang.org/grpc"
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"
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
	sipRaw, ok := ctx.Get("sipraw")
	if !ok {
		return fmt.Errorf("SIP raw data not found in context")
	}
	sipData, ok := sipRaw.(*v1.SipRaw)
	if !ok {
		return fmt.Errorf("invalid SIP raw data type")
	}

	msg := &sarama.ProducerMessage{
		Topic: f.Topic,
		Value: sarama.ByteEncoder(sipData.Data),
	}

	partition, offset, err := f.producer.SendMessage(msg)
	if err != nil {
		return err
	}

	ctx.Info("SIP raw data sent to Kafka", "partition", partition, "offset", offset)
	return nil
}

func (f *Forwarder) ForwardType() v1.SniffType {
	return v1.SniffType_SipRaw
}

func (f *Forwarder) SyncForward(_ *v1.SniffData) (*v1.SniffData, grpc.ClientStream, error) {
	return nil, nil, fmt.Errorf("unsupport sync forward")
}

func (f *Forwarder) SupportedSyncInvoke() bool {
	return false
}
