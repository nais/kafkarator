package kafka

import (
	"github.com/Shopify/sarama"
	"github.com/nais/kafkarator/pkg/crypto"
	log "github.com/sirupsen/logrus"
)

type CryptInterceptor struct {
	Key    []byte
	Logger *log.Logger
}

func (c *CryptInterceptor) OnConsume(msg *sarama.ConsumerMessage) {
	plaintext, err := crypto.Decrypt(msg.Value, c.Key)
	if err != nil {
		log.Errorf("unable to decrypt incoming Kafka message: %s", err)
	}
	msg.Value = plaintext
}

func (c *CryptInterceptor) OnSend(msg *sarama.ProducerMessage) {
	plaintext, err := msg.Value.Encode()
	if err == nil {
		ciphertext, err := crypto.Encrypt(plaintext, c.Key)
		if err != nil {
			log.Errorf("unable to encrypt outgoing Kafka message; sending empty string instead: %s", err)
			ciphertext = make([]byte, 0)
		}
		msg.Value = sarama.ByteEncoder(ciphertext)
		return
	}
	log.Errorf("crypt interceptor: encoding error: %s")
}
