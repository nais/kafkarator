package kafka

import (
	"fmt"
	"github.com/Shopify/sarama"
	log "github.com/sirupsen/logrus"
	"time"
)

type Callback struct {
	slow bool
	cons chan<- Message
}

func NewCallback(slow bool, c chan<- Message) *Callback {
	return &Callback{
		slow: slow,
		cons: c,
	}
}

func (c *Callback) Callback(msg *sarama.ConsumerMessage, logger *log.Entry) (bool, error) {
	t, err := time.Parse(time.RFC3339Nano, string(msg.Value))
	if err != nil {
		return false, fmt.Errorf("converting string to timestamp: %s", err)
	}
	message := Message{
		Offset:    msg.Offset,
		TimeStamp: t,
		Partition: msg.Partition,
	}
	if c.slow {
		logger.Infof("Slow consumer is sleepy...")
		time.Sleep(time.Second * 10)
	}
	logger.Infof("Consumed message: %s", message.String())
	c.cons <- message

	return false, nil
}
