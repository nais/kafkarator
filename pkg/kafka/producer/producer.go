package producer

import (
	"crypto/tls"
	"fmt"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
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

func New(brokers []string, topic string, txProducer bool, tlsConfig *tls.Config, logger *log.Logger) (*Producer, error) {
	config := sarama.NewConfig()
	config.Net.TLS.Enable = true
	config.Net.TLS.Config = tlsConfig
	config.Version = sarama.V3_1_0_0
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Errors = true
	config.Producer.Return.Successes = true
	config.ClientID, _ = os.Hostname()
	sarama.Logger = logger

	if txProducer {
		uuidStr := uuid.NewString()
		config.Producer.Transaction.ID = fmt.Sprintf("%s_%s", topic, uuidStr) // these need to be unique, in some sense
		config.Producer.Idempotent = true
		config.Net.MaxOpenRequests = 1
	}
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
	for range retryCount {
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
