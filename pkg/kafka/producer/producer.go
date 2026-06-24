package producer

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/nais/kafkarator/pkg/kafka"
	log "github.com/sirupsen/logrus"
)

// ErrFatalProducer is returned by ProduceTx when the underlying Sarama producer
// has entered a fatal or permanently-stuck state and must be closed and recreated
// before any further transactions can succeed.
var ErrFatalProducer = errors.New("transactional producer is in a fatal state and must be recreated")

type Producer struct {
	producer   sarama.SyncProducer
	topic      string
	logger     *log.Logger
	brokers    []string
	txProducer bool
	tlsConfig  *tls.Config
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
		config.Producer.Transaction.ID = uuidStr // these need to be unique, in some sense
		config.Producer.Idempotent = true
		config.Net.MaxOpenRequests = 1
	}
	p, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, err
	}

	return &Producer{
		producer:   p,
		topic:      topic,
		logger:     logger,
		brokers:    brokers,
		txProducer: txProducer,
		tlsConfig:  tlsConfig,
	}, nil
}

// Recreate closes the current underlying producer and creates a fresh one with a
// new transaction ID. It replaces the receiver in place so callers holding a
// *Producer pointer automatically use the new connection.
func (p *Producer) Recreate() error {
	p.logger.Warnf("Recreating transactional producer after fatal state")
	if err := p.producer.Close(); err != nil {
		p.logger.Warnf("Error closing broken producer (ignoring): %s", err)
	}
	fresh, err := New(p.brokers, p.topic, p.txProducer, p.tlsConfig, p.logger)
	if err != nil {
		return fmt.Errorf("recreating producer: %w", err)
	}
	p.producer = fresh.producer
	return nil
}

// isFatal returns true when the transaction manager is in a state from which it
// cannot recover without a full Close + recreate cycle.
func (p *Producer) isFatal() bool {
	status := p.producer.TxnStatus()
	return status&sarama.ProducerTxnFlagFatalError != 0 ||
		status == sarama.ProducerTxnFlagUninitialized
}

func (p *Producer) Produce(msg kafka.Message) (int32, int64, error) {
	producerMessage := &sarama.ProducerMessage{
		Topic:     p.topic,
		Value:     sarama.ByteEncoder(msg),
		Timestamp: time.Now(),
	}
	return p.producer.SendMessage(producerMessage)
}

func (p *Producer) ProduceTx(msg []kafka.Message) (int32, int64, error) {
	if p.isFatal() {
		return 0, 0, ErrFatalProducer
	}

	p.logger.Infof("Starting transaction for %d messages", len(msg))
	err := p.producer.BeginTxn()
	if err != nil {
		p.logger.Errorf("Failed to begin transaction: %s", err)
		if p.isFatal() {
			return 0, 0, fmt.Errorf("%w: %s", ErrFatalProducer, err)
		}
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
				p.logger.Errorf("Failed to abort transaction: %s", abortErr)
				return 0, 0, fmt.Errorf("%w: produce error: %s, abort txn error: %s", ErrFatalProducer, err, abortErr)
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
		// Check after each failed commit whether the state has become fatal.
		if p.isFatal() {
			p.logger.Errorf("Producer entered fatal state during commit: %s", err)
			return 0, 0, fmt.Errorf("%w: commit error: %s", ErrFatalProducer, err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	p.logger.Errorf("Failed to commit transaction after %d retries: %s", retryCount, err)
	abortErr := p.producer.AbortTxn()
	if abortErr != nil {
		p.logger.Errorf("Failed to abort transaction: %s", abortErr)
		return 0, 0, fmt.Errorf("%w: commit error: %s, abort txn error: %s", ErrFatalProducer, err, abortErr)
	}
	return 0, 0, fmt.Errorf("commit error: %w", err)
}
