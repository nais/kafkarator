package consumer

import (
	"context"
	"crypto/tls"
	"os"
	"time"

	"github.com/nais/kafkarator/pkg/kafka"

	"github.com/IBM/sarama"
	"github.com/nais/kafkarator/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type Callback func(message *sarama.ConsumerMessage, logger *log.Entry) (retry bool, err error)

type Consumer struct {
	callback      Callback
	consumer      sarama.ConsumerGroup
	groupID       string
	logger        *log.Logger
	retryInterval time.Duration
	topic         string
}

type Config struct {
	Brokers           []string
	Callback          Callback
	GroupID           string
	MaxProcessingTime time.Duration
	Logger            *log.Logger
	RetryInterval     time.Duration
	TlsConfig         *tls.Config
	Topic             string
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (c *Consumer) Setup(_ sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (c *Consumer) Cleanup(_ sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	retry := true
	var err error
	groupId := c.topic + "-" + c.groupID

	report := func(add int) {
		metrics.SecretQueueSize.With(prometheus.Labels{
			metrics.LabelGroupID: groupId,
		}).Set(float64(len(claim.Messages()) + add))
	}
	report(0)

	for message := range claim.Messages() {
		for retry {
			report(1)
			logger := c.logger.WithFields(log.Fields{
				"kafka_offset": message.Offset,
			})
			retry, err = c.callback(message, logger)
			if err != nil {
				logger.Errorf("Consume Kafka message: %s", err)
				if retry {
					time.Sleep(c.retryInterval)
				}
			}
		}
		retry, err = true, nil
		session.MarkMessage(message, "")
		report(0)
	}
	return nil
}

func New(ctx context.Context, cancel context.CancelFunc, cfg Config) error {
	config := sarama.NewConfig()
	config.Net.TLS.Enable = true
	config.Net.TLS.Config = cfg.TlsConfig
	config.Version = sarama.V3_1_0_0
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.MaxProcessingTime = cfg.MaxProcessingTime
	config.ClientID, _ = os.Hostname()
	config.Consumer.Return.Errors = true
	sarama.Logger = cfg.Logger
	groupId := cfg.Topic + "-" + cfg.GroupID

	consumer, err := sarama.NewConsumerGroup(cfg.Brokers, groupId, config)
	if err != nil {
		return err
	}

	c := &Consumer{
		callback:      cfg.Callback,
		consumer:      consumer,
		groupID:       groupId,
		logger:        cfg.Logger,
		retryInterval: cfg.RetryInterval,
		topic:         cfg.Topic,
	}

	go func() {
		for err := range c.consumer.Errors() {
			c.logger.Errorf("Consumer encountered error: %s", err)
			if kafka.IsErrUnauthorized(err) {
				c.logger.Errorf("credentials rotated or invalidated")
				cancel()
				return
			}
		}
	}()

	go func() {
		for ctx.Err() == nil {
			c.logger.Infof("(re-)starting consumer on topic %s", cfg.Topic)
			err := c.consumer.Consume(ctx, []string{cfg.Topic}, c)
			if err != nil {
				c.logger.Errorf("Error setting up consumer: %s", err)
			}
			time.Sleep(10 * time.Second)
		}
	}()

	return nil
}
