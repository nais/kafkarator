package producer

import (
	"crypto/tls"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/nais/kafkarator/pkg/kafka"
	log "github.com/sirupsen/logrus"
)

type Producer struct {
	producer sarama.SyncProducer
	topic    string
}

type Interface interface {
	Produce(msg kafka.Message) (partition int32, offset int64, err error)
}

func New(brokers []string, topic string, tlsConfig *tls.Config, logger *log.Logger) (*Producer, error) {
	config := sarama.NewConfig()
	config.Net.TLS.Enable = true
	config.Net.TLS.Config = tlsConfig
	config.Version = sarama.V3_1_0_0
	// V ????
	config.Producer.Transaction.ID = "canary"
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Errors = true
	config.Producer.Return.Successes = true
	config.ClientID, _ = os.Hostname()
	sarama.Logger = logger

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}

	return &Producer{
		producer: producer,
		topic:    topic,
	}, nil
}

func (p *Producer) Produce(msg kafka.Message) (int32, int64, error) {
	producerMessage := &sarama.ProducerMessage{
		Topic:     p.topic,
		Value:     sarama.ByteEncoder(msg),
		Timestamp: time.Now(),
	}
	return p.producer.SendMessage(producerMessage)
}
