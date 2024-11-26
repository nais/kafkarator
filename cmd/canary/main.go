package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nais/kafkarator/pkg/canary/certificates"
	canarykafka "github.com/nais/kafkarator/pkg/canary/kafka"
	"github.com/nais/kafkarator/pkg/kafka"
	"github.com/nais/kafkarator/pkg/kafka/consumer"
	"github.com/nais/kafkarator/pkg/kafka/producer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	ExitOK = iota
	ExitConfig
	ExitRuntime

	Namespace = "kafkarator_canary"
)

// Configuration options
const (
	KafkaBrokers           = "kafka-brokers"
	KafkaCAPath            = "kafka-ca-path"
	KafkaCertificatePath   = "kafka-certificate-path"
	KafkaGroupID           = "kafka-group-id"
	KafkaKeyPath           = "kafka-key-path"
	KafkaTopic             = "kafka-topic"
	DeployStartTime        = "deploy-start-time"
	LogFormat              = "log-format"
	MessageInterval        = "message-interval"
	MetricsAddress         = "metrics-address"
	SlowConsumer           = "slow-consumer"
	KafkaTransactionTopic  = "kafka-tx-topic"
	KafkaTransactionEnable = "enable-transaction"
)

const (
	LogFormatJSON = "json"
	LogFormatText = "text"
)

var (
	LeadTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "lead_time",
		Namespace: Namespace,
		Help:      "seconds used in deployment pipeline, from making the request until the application is available",
	})

	DeployTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "deploy_timestamp",
		Namespace: Namespace,
		Help:      "timestamp when the deploy of this application was triggered in the pipeline",
	})

	StartTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "start_timestamp",
		Namespace: Namespace,
		Help:      "start time of the application",
	})

	LastProducedTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "last_produced",
		Help:      "timestamp of last produced canary message",
	})

	LastConsumedTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "last_consumed",
		Help:      "timestamp of last consumed canary message",
	})

	LastProducedOffset = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "last_produced_offset",
		Help:      "offset of last produced canary message",
	})

	LastConsumedOffset = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "last_consumed_offset",
		Help:      "offset of last consumed canary message",
	})

	ProduceLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:      "produce_latency",
		Namespace: Namespace,
		Help:      "latency in message production",
		Buckets:   prometheus.LinearBuckets(0.01, 0.01, 100),
	})

	ProduceTxLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:      "produce_tx_latency",
		Namespace: Namespace,
		Help:      "latency in message production",
		Buckets:   prometheus.LinearBuckets(0.01, 0.01, 100),
	})

	ConsumeLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:      "consume_latency",
		Namespace: Namespace,
		Help:      "latency in message consumption",
		Buckets:   prometheus.LinearBuckets(0.01, 0.01, 100),
	})

	TransactionTxLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:      "transaction_latency",
		Namespace: Namespace,
		Help:      "latency in transactional message consumption",
		Buckets:   prometheus.LinearBuckets(0.01, 0.01, 100),
	})

	TransactedOffset = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "transacted_messages_total",
		Namespace: Namespace,
		Help:      "transacted messages, transcations happen in units of 100 messages in the canary",
	})
	LastConsumedTxTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "last_consumed_tx",
		Help:      "timestamp of last consumed transaction",
	})
)

func init() {
	// Automatically read configuration options from environment variables.
	// i.e. --aiven-token will be configurable using CANARY_AIVEN_TOKEN.
	viper.SetEnvPrefix("CANARY")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	flag.String(MetricsAddress, "127.0.0.1:8080", "The address the metric endpoint binds to.")
	flag.String(LogFormat, "text", "Log format, either 'text' or 'json'")

	flag.Duration(MessageInterval, time.Minute*1, "Interval between each produced canary message to Kafka")
	flag.String(DeployStartTime, time.Now().Format(time.RFC3339), "RFC3339 formatted time of deploy")

	flag.Bool(SlowConsumer, false, "Simulate a slow consumer by sleeping for 10 seconds after each consumed message")

	// Kafka configuration
	hostname, _ := os.Hostname()
	flag.StringSlice(KafkaBrokers, []string{"localhost:9092"}, "Broker addresses for Kafka support")
	flag.String(KafkaTopic, "kafkarator-canary", "Topic where Kafkarator canary messages are produced")
	flag.String(KafkaTransactionTopic, "kafkarator-tx-canary", "Topic where Kafkarator canary messages are transcated between")
	flag.String(KafkaTransactionEnable, "kafkarator-canary-transactions-enable", "Enable transactions canarying")
	flag.String(KafkaGroupID, hostname, "Kafka group ID for storing consumed message positions")
	flag.String(KafkaCertificatePath, "kafka.crt", "Path to Kafka client certificate")
	flag.String(KafkaKeyPath, "kafka.key", "Path to Kafka client key")
	flag.String(KafkaCAPath, "ca.crt", "Path to Kafka CA certificate")

	// Read config from NAIS
	// https://doc.nais.io/addons/kafka#application-config
	_ = viper.BindEnv(KafkaBrokers, "KAFKA_BROKERS")
	_ = viper.BindEnv(KafkaCertificatePath, "KAFKA_CERTIFICATE_PATH")
	_ = viper.BindEnv(KafkaKeyPath, "KAFKA_PRIVATE_KEY_PATH")
	_ = viper.BindEnv(KafkaCAPath, "KAFKA_CA_PATH")

	flag.Parse()

	err := viper.BindPFlags(flag.CommandLine)
	if err != nil {
		panic(err)
	}

	prometheus.MustRegister(
		ConsumeLatency,
		DeployTimestamp,
		LastConsumedOffset,
		LastConsumedTimestamp,
		LastConsumedTxTimestamp,
		LastProducedOffset,
		LastProducedTimestamp,
		LeadTime,
		ProduceLatency,
		ProduceTxLatency,
		StartTimestamp,
		TransactedOffset,
		TransactionTxLatency,
	)
}

func formatter(logFormat string) (log.Formatter, error) {
	switch logFormat {
	case LogFormatJSON:
		return &log.JSONFormatter{
			TimestampFormat:   time.RFC3339Nano,
			DisableHTMLEscape: true,
		}, nil
	case LogFormatText:
		return &log.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339Nano,
		}, nil
	}
	return nil, fmt.Errorf("unsupported log format '%s'", logFormat)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	signals := make(chan os.Signal, 1)
	cons := make(chan canarykafka.Message, 32)
	consTx := make(chan canarykafka.Message, 32)

	logger := log.New()
	logfmt, err := formatter(viper.GetString(LogFormat))
	if err != nil {
		logger.Error(err)
		os.Exit(ExitConfig)
	}

	logger.SetFormatter(logfmt)

	logger.Infof("kafkarator-canary starting up...")

	err = recordStartupTimes()
	if err != nil {
		logger.Error(err)
		os.Exit(ExitConfig)
	}

	go func() {
		logger.Error(http.ListenAndServe(viper.GetString(MetricsAddress), promhttp.Handler()))
		cancel()
	}()

	certBundle, err := certificates.New(viper.GetString(KafkaCertificatePath), viper.GetString(KafkaKeyPath), viper.GetString(KafkaCAPath))
	if err != nil {
		logger.Errorf("unable to read TLS config: %s", err)
		os.Exit(ExitConfig)
	}

	tlsConfig, err := kafka.TLSConfig(certBundle.Cert, certBundle.Key, certBundle.Ca)
	if err != nil {
		logger.Errorf("unable to set up Kafka TLS config: %s", err)
		os.Exit(ExitConfig)
	}

	prodtx, err := producer.New(viper.GetStringSlice(KafkaBrokers), viper.GetString(KafkaTopic), true, tlsConfig, logger)
	if err != nil {
		logger.Errorf("unable to set up kafka producer: %s", err)
		os.Exit(ExitConfig)
	}

	prod, err := producer.New(viper.GetStringSlice(KafkaBrokers), viper.GetString(KafkaTopic), false, tlsConfig, logger)
	if err != nil {
		logger.Errorf("unable to set up kafka producer: %s", err)
		os.Exit(ExitConfig)
	}

	logger.Infof("Started message producer.")

	txCallback := canarykafka.NewCallback(false, consTx)
	err = consumer.New(ctx, cancel, consumer.Config{
		Brokers:           viper.GetStringSlice(KafkaBrokers),
		GroupID:           viper.GetString(KafkaGroupID),
		MaxProcessingTime: time.Second * 1,
		RetryInterval:     time.Second * 10,
		Topic:             viper.GetString(KafkaTransactionTopic),
		Callback:          txCallback.Callback, // May or may not need to have special here
		Logger:            logger,
		TlsConfig:         tlsConfig,
	})

	callback := canarykafka.NewCallback(viper.GetBool(SlowConsumer), cons)
	err = consumer.New(ctx, cancel, consumer.Config{
		Brokers:           viper.GetStringSlice(KafkaBrokers),
		GroupID:           viper.GetString(KafkaGroupID),
		MaxProcessingTime: time.Second * 1,
		RetryInterval:     time.Second * 10,
		Topic:             viper.GetString(KafkaTopic),
		Callback:          callback.Callback,
		Logger:            logger,
		TlsConfig:         tlsConfig,
	})
	if err != nil {
		logger.Errorf("unable to set up kafka consumer: %s", err)
		cancel()
		os.Exit(ExitConfig)
	}

	logger.Infof("Started message consumer.")

	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

	produce := func(ctx context.Context) {
		if ctx.Err() != nil {
			return
		}
		timer := time.Now()
		partition, offset, err := prod.Produce(kafka.Message(timer.Format(time.RFC3339Nano)))
		ProduceLatency.Observe(time.Now().Sub(timer).Seconds())
		if err == nil {
			message := canarykafka.Message{
				Offset:    offset,
				TimeStamp: timer,
				Partition: partition,
			}
			logger.Infof("Produced message: %s", message.String())
			LastProducedTimestamp.SetToCurrentTime()
			LastProducedOffset.Set(float64(offset))
		} else {
			logger.Errorf("unable to produce canary message on Kafka: %s", err)
			if kafka.IsErrUnauthorized(err) {
				cancel()
			}
		}
	}

	produceTx := func(ctx context.Context) {
		if ctx.Err() != nil {
			return
		}
		timer := time.Now()
		var messages []kafka.Message
		for i := 0; i < 5; i++ {
			messages = append(messages, kafka.Message(timer.Format(time.RFC3339Nano)))
		}
		_, offset, err := prodtx.ProduceTx(messages)
		ProduceTxLatency.Observe(time.Now().Sub(timer).Seconds())
		if err == nil {
			logger.Infof("Produced transaction")
			TransactionTxLatency.Observe(time.Now().Sub(timer).Seconds())
			//	TransactedOffset.Set(float64(offset))
		} else {
			logger.Errorf("unable to produce transaction on Kafka: %s", err)
			if kafka.IsErrUnauthorized(err) {
				cancel()
			}
		}
	}

	logger.Infof("Ready.")

	produceTicker := time.NewTicker(viper.GetDuration(MessageInterval))
	produceTxTicker := time.NewTicker(viper.GetDuration(MessageInterval) + time.Second*10)

	for ctx.Err() == nil {
		select {
		case <-produceTicker.C:
			certBundle.DiffAndUpdate(logger)
			produce(ctx)
		case msg := <-cons:
			LastConsumedTimestamp.SetToCurrentTime()
			ConsumeLatency.Observe(time.Now().Sub(msg.TimeStamp).Seconds())
			LastConsumedOffset.Set(float64(msg.Offset))
		case <-produceTxTicker.C:
			produceTx(ctx)
		case msg := <-consTx:
			LastConsumedTxTimestamp.SetToCurrentTime()
			TransactionTxLatency.Observe(time.Now().Sub(msg.TimeStamp).Seconds())
		// 	TransactedOffset.Set(float64(msg.Offset))
		case sig := <-signals:
			logger.Infof("exiting due to signal: %s", strings.ToUpper(sig.String()))
			cancel()
			os.Exit(ExitOK)
		}
	}

	cancel()
	logger.Errorf("quit: %s", ctx.Err())
	os.Exit(ExitRuntime)
}

func recordStartupTimes() error {
	deployStartTime, err := time.Parse(time.RFC3339, viper.GetString(DeployStartTime))
	if err != nil {
		return err
	}

	StartTimestamp.SetToCurrentTime()
	DeployTimestamp.Set(float64(deployStartTime.Unix()))
	LeadTime.Set(time.Now().Sub(deployStartTime).Seconds())
	return nil
}
