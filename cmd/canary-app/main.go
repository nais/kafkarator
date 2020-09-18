package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Shopify/sarama"
	"github.com/nais/kafkarator/pkg/kafka"
	"github.com/nais/kafkarator/pkg/kafka/consumer"
	"github.com/nais/kafkarator/pkg/kafka/producer"
	"github.com/nais/kafkarator/pkg/utils"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type QuitChannel chan error

const (
	ExitOK = iota
	ExitRuntime
)

// Configuration options
const (
	KafkaBrokers         = "kafka-brokers"
	KafkaCAPath          = "kafka-ca-path"
	KafkaCertificatePath = "kafka-certificate-path"
	KafkaGroupID         = "kafka-group-id"
	KafkaKeyPath         = "kafka-key-path"
	KafkaTopic           = "kafka-topic"
	LogFormat            = "log-format"
	MetricsAddress       = "metrics-address"
)

func init() {
	// Automatically read configuration options from environment variables.
	// i.e. --aiven-token will be configurable using KAFKARATOR_AIVEN_TOKEN.
	viper.SetEnvPrefix("KAFKARATOR")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	flag.String(MetricsAddress, "127.0.0.1:8080", "The address the metric endpoint binds to.")
	flag.String(LogFormat, "text", "Log format, either 'text' or 'json'")

	// Kafka configuration
	hostname, _ := os.Hostname()
	flag.StringSlice(KafkaBrokers, []string{"localhost:9092"}, "Broker addresses for Kafka support")
	flag.String(KafkaTopic, "kafkarator-canary", "Topic where Kafkarator canary messages are produced")
	flag.String(KafkaGroupID, hostname, "Kafka group ID for storing consumed message positions")
	flag.String(KafkaCertificatePath, "kafka.crt", "Path to Kafka client certificate")
	flag.String(KafkaKeyPath, "kafka.key", "Path to Kafka client key")
	flag.String(KafkaCAPath, "ca.crt", "Path to Kafka CA certificate")

	flag.Parse()

	err := viper.BindPFlags(flag.CommandLine)
	if err != nil {
		panic(err)
	}
}

func main() {
	quit := make(QuitChannel, 1)
	signals := make(chan os.Signal, 1)
	consume := make(chan time.Time, 32)
	logger := log.New()

	cert, key, ca, err := utils.TlsFromFiles(viper.GetString(KafkaCertificatePath), viper.GetString(KafkaKeyPath), viper.GetString(KafkaCAPath))
	if err != nil {
		quit <- fmt.Errorf("unable to set up TLS config: %s", err)
		return
	}

	tlsConfig, err := kafka.TLSConfig(cert, key, ca)
	if err != nil {
		quit <- fmt.Errorf("unable to set up TLS config: %s", err)
		return
	}

	prod, err := producer.New(viper.GetStringSlice(KafkaBrokers), viper.GetString(KafkaTopic), tlsConfig, logger, nil)
	if err != nil {
		quit <- fmt.Errorf("unable to set up kafka producer: %s", err)
		return
	}

	callback := func(msg *sarama.ConsumerMessage, logger *log.Entry) (bool, error) {
		t, err := time.Parse(time.RFC3339Nano, string(msg.Value))
		if err != nil {
			return false, fmt.Errorf("converting string to timestamp: %s", err)
		}
		consume <- t

		return false, nil
	}

	_, err = consumer.New(consumer.Config{
		Brokers:           viper.GetStringSlice(KafkaBrokers),
		GroupID:           viper.GetString(KafkaGroupID),
		MaxProcessingTime: time.Second * 1,
		RetryInterval:     time.Second * 10,
		Topic:             viper.GetString(KafkaTopic),
		Callback:          callback,
		Logger:            logger,
		TlsConfig:         tlsConfig,
	})
	if err != nil {
		quit <- fmt.Errorf("unable to set up kafka consumer: %s", err)
	}

	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		for {
			offset, err := prod.Produce(kafka.Message(time.Now().Format(time.RFC3339Nano)))
			if err != nil {
				quit <- fmt.Errorf("unable to produce canary message on Kafka: %w", err)
				return
			}
			logger.Info("Produced canary message at offset ", offset)
			time.Sleep(time.Minute * 1)
		}
	}()

	for {
		select {
		case msg := <-consume:
			logger.Infof("consumed message with timestamp: %s", msg)
		case err := <-quit:
			logger.Errorf("terminating unexpectedly: %s", err)
			os.Exit(ExitRuntime)
		case sig := <-signals:
			logger.Infof("exiting due to signal: %s", strings.ToUpper(sig.String()))
			os.Exit(ExitOK)
		}
	}
}
