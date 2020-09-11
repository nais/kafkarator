package consumer

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/Shopify/sarama"
	"github.com/nais/kafkarator/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type Callback func(message *sarama.ConsumerMessage, logger *log.Entry) (retry bool, err error)

type Consumer struct {
	callback      Callback
	cancel        context.CancelFunc
	consumer      sarama.ConsumerGroup
	ctx           context.Context
	groupID       string
	logger        *log.Logger
	retryInterval time.Duration
	topic         string
}

type Config struct {
	Brokers       []string
	Callback      Callback
	GroupID       string
	Interceptor   sarama.ConsumerInterceptor
	Logger        *log.Logger
	RetryInterval time.Duration
	TlsConfig     *tls.Config
	Topic         string
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
	var retry = true
	var err error

	report := func(add int) {
		metrics.SecretQueueSize.With(prometheus.Labels{
			metrics.LabelGroupID: c.groupID,
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
				time.Sleep(c.retryInterval)
			}
		}
		retry, err = true, nil
		session.MarkMessage(message, "")
		report(0)
	}
	return nil
}

func New(cfg Config) (*Consumer, error) {
	config := sarama.NewConfig()
	config.Net.TLS.Enable = true
	config.Net.TLS.Config = cfg.TlsConfig
	config.Version = sarama.V2_6_0_0
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategySticky
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Interceptors = []sarama.ConsumerInterceptor{cfg.Interceptor}
	sarama.Logger = cfg.Logger

	consumer, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.GroupID, config)
	if err != nil {
		return nil, err
	}

	c := &Consumer{
		callback:      cfg.Callback,
		consumer:      consumer,
		groupID:       cfg.GroupID,
		logger:        cfg.Logger,
		retryInterval: cfg.RetryInterval,
		topic:         cfg.Topic,
	}

	c.ctx, c.cancel = context.WithCancel(context.Background())

	go func() {
		for err := range c.consumer.Errors() {
			c.logger.Errorf("Consumer encountered error: %s", err)
		}
	}()

	go func() {
		for {
			c.logger.Infof("(re-)starting consumer on topic %s", cfg.Topic)
			err := c.consumer.Consume(c.ctx, []string{cfg.Topic}, c)
			if err != nil {
				c.logger.Errorf("Error setting up consumer: %s", err)
			}
			// check if context was cancelled, signaling that the consumer should stop
			if c.ctx.Err() != nil {
				c.logger.Errorf("Consumer context error: %s", c.ctx.Err())
				c.ctx, c.cancel = context.WithCancel(context.Background())
			}
			time.Sleep(10 * time.Second)
		}
	}()

	return c, nil
}
