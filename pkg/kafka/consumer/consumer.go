package consumer

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"github.com/nais/kafkarator/pkg/kafka"
	log "github.com/sirupsen/logrus"
)

type Consumer struct {
	Messages chan kafka.Message
	consumer sarama.ConsumerGroup
	topic    string
	ctx      context.Context
	cancel   context.CancelFunc
	logger   *log.Logger
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
	for message := range claim.Messages() {
		msg := fmt.Sprintf("Message claimed: value = %s, timestamp = %v, topic = %s", string(message.Value), message.Timestamp, message.Topic)
		session.MarkMessage(message, "")
		c.Messages <- kafka.Message(msg)
	}
	return nil
}

func New(brokers []string, topic, groupID string, tlsConfig *tls.Config, logger *log.Logger) (*Consumer, error) {
	config := sarama.NewConfig()
	config.Net.TLS.Enable = true
	config.Net.TLS.Config = tlsConfig
	config.Version = sarama.V2_6_0_0
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategySticky
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	sarama.Logger = logger

	consumer, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, err
	}

	c := &Consumer{
		Messages: make(chan kafka.Message, 1024),
		consumer: consumer,
		topic:    topic,
		logger:   logger,
	}

	c.ctx, c.cancel = context.WithCancel(context.Background())

	go func() {
		for err := range c.consumer.Errors() {
			logger.Errorf("Consumer encountered error: %s", err)
		}
	}()

	go func() {
		for {
			logger.Infof("(re-)starting consumer on topic %s", topic)
			err := c.consumer.Consume(c.ctx, []string{topic}, c)
			if err != nil {
				logger.Errorf("Error setting up consumer: %s", err)
			}
			// check if context was cancelled, signaling that the consumer should stop
			if c.ctx.Err() != nil {
				logger.Errorf("Consumer context error: %s", c.ctx.Err())
				c.ctx, c.cancel = context.WithCancel(context.Background())
			}
			time.Sleep(10 * time.Second)
		}
	}()

	return c, nil
}
