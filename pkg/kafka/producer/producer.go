package producer

import (
	"crypto/tls"
	"fmt"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/nais/kafkarator/pkg/kafka"
	log "github.com/sirupsen/logrus"
)

type Producer struct {
	producer sarama.SyncProducer
	topic    string
	logger   *log.Logger
}

type Interface interface {
	Produce(msg kafka.Message) (partition int32, offset int64, err error)
	ProduceTx(msg []kafka.Message) (partition int32, offset int64, err error)
}

func New(brokers []string, topic, producerId string, tlsConfig *tls.Config, logger *log.Logger) (*Producer, error) {
	config := sarama.NewConfig()
	config.Net.TLS.Enable = true
	config.Net.TLS.Config = tlsConfig
	config.Version = sarama.V3_1_0_0
	// V ????
	config.Producer.Transaction.ID = producerId
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Errors = true
	config.Producer.Return.Successes = true
	config.Producer.Idempotent = true
	config.Net.MaxOpenRequests = 1
	config.ClientID, _ = os.Hostname()
	sarama.Logger = logger

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}

	return &Producer{
		producer: producer,
		topic:    topic,
		logger:   logger,
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

func (p *Producer) Close() error {
	return p.producer.Close()
}
func (p *Producer) ProduceTx(msg []kafka.Message) (int32, int64, error) {
	p.logger.Infof("Starting transaction for %d messages", len(msg))
	err := p.producer.BeginTxn()
	if err != nil {
		p.logger.Errorf("Failed to begin transaction: %s", err)
		return 0, 0, err
	}

	var partition int32
	var offset int64
	for i, m := range msg {
		partition, offset, err = p.Produce(m)
		if err != nil {
			p.logger.Errorf("Failed to produce message %d: %s", i, err)
			abortErr := p.producer.AbortTxn()
			if abortErr != nil {
				return 0, 0, fmt.Errorf("produce error: %w, abort txn error: %s", err, abortErr)
			}
			return 0, 0, err
		}
	}

	retryCount := 3
	for i := 0; i < retryCount; i++ {
		err = p.producer.CommitTxn()
		if err == nil {
			p.logger.Infof("Transaction committed successfully")
			return partition, offset, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	p.logger.Errorf("Failed to commit transaction after %d retries: %s", retryCount, err)
	abortErr := p.producer.AbortTxn()
	if abortErr != nil {
		return 0, 0, fmt.Errorf("commit error: %w, abort txn error: %s", err, abortErr)
	}
	return 0, 0, fmt.Errorf("commit error: %w", err)
}
