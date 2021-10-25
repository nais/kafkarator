package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Shopify/sarama"
	"github.com/nais/kafkarator/pkg/kafka"
	"github.com/nais/kafkarator/pkg/kafka/consumer"
	"github.com/nais/kafkarator/pkg/kafka/producer"
	"github.com/nais/kafkarator/pkg/utils"
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
	KafkaBrokers         = "kafka-brokers"
	KafkaCAPath          = "kafka-ca-path"
	KafkaCertificatePath = "kafka-certificate-path"
	KafkaGroupID         = "kafka-group-id"
	KafkaKeyPath         = "kafka-key-path"
	KafkaTopic           = "kafka-topic"
	LogFormat            = "log-format"
	MessageInterval      = "message-interval"
	MetricsAddress       = "metrics-address"
)

const (
	LogFormatJSON = "json"
	LogFormatText = "text"
)

type Consume struct {
	offset    int64
	timeStamp time.Time
}

var (
	deployStartTimestamp int64

	LeadTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "lead_time",
		Namespace: Namespace,
		Help:      "seconds used in deployment pipeline, from making the request until the application is available",
	})

	TimeSinceDeploy = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "time_since_deploy",
		Namespace: Namespace,
		Help:      "seconds since the latest deploy of this application",
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

	ConsumeLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:      "consume_latency",
		Namespace: Namespace,
		Help:      "latency in message consumption",
		Buckets:   prometheus.LinearBuckets(0.01, 0.01, 100),
	})
)

func init() {
	// Automatically read configuration options from environment variables.
	// i.e. --aiven-token will be configurable using KAFKARATOR_AIVEN_TOKEN.
	viper.SetEnvPrefix("CANARY")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	flag.String(MetricsAddress, "127.0.0.1:8080", "The address the metric endpoint binds to.")
	flag.String(LogFormat, "text", "Log format, either 'text' or 'json'")

	flag.Duration(MessageInterval, time.Minute*1, "Interval between each produced canary message to Kafka")
	flag.Int64Var(&deployStartTimestamp, "deploy-start-time", getEnvInt("DEPLOY_START", time.Now().UnixNano()), "unix timestamp with nanoseconds, specifies when NAIS deploy of testapp started")

	// Kafka configuration
	hostname, _ := os.Hostname()
	flag.StringSlice(KafkaBrokers, []string{"localhost:9092"}, "Broker addresses for Kafka support")
	flag.String(KafkaTopic, "kafkarator-canary", "Topic where Kafkarator canary messages are produced")
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
		LastProducedOffset,
		LastProducedTimestamp,
		LeadTime,
		ProduceLatency,
		StartTimestamp,
		TimeSinceDeploy,
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

func getEnvInt(key string, fallback int64) int64 {
	if value, ok := os.LookupEnv(key); ok {
		i, _ := strconv.ParseInt(value, 10, 64)
		return i
	}

	return fallback
}

func timeSinceDeploy() float64 {
	deployStartTime := time.Unix(0, deployStartTimestamp)
	return time.Now().Sub(deployStartTime).Seconds()
}

func main() {
	type QuitChannel chan error

	quit := make(QuitChannel, 1)
	signals := make(chan os.Signal, 1)
	cons := make(chan Consume, 32)

	logger := log.New()
	logfmt, err := formatter(viper.GetString(LogFormat))
	if err != nil {
		logger.Error(err)
		os.Exit(ExitConfig)
	}

	logger.SetFormatter(logfmt)

	logger.Infof("kafkarator-canary starting up...")

	StartTimestamp.SetToCurrentTime()
	DeployTimestamp.Set(float64(deployStartTimestamp) / 10e8)
	LeadTime.Set(timeSinceDeploy())
	TimeSinceDeploy.Set(timeSinceDeploy())

	go func() {
		quit <- http.ListenAndServe(viper.GetString(MetricsAddress), promhttp.Handler())
	}()

	cert, key, ca, err := utils.TlsFromFiles(viper.GetString(KafkaCertificatePath), viper.GetString(KafkaKeyPath), viper.GetString(KafkaCAPath))
	if err != nil {
		logger.Errorf("unable to set up TLS config: %s", err)
		return
	}

	tlsConfig, err := kafka.TLSConfig(cert, key, ca)
	if err != nil {
		logger.Errorf("unable to set up TLS config: %s", err)
		return
	}

	prod, err := producer.New(viper.GetStringSlice(KafkaBrokers), viper.GetString(KafkaTopic), tlsConfig, logger)
	if err != nil {
		logger.Errorf("unable to set up kafka producer: %s", err)
		return
	}

	logger.Infof("Started message producer.")

	callback := func(msg *sarama.ConsumerMessage, logger *log.Entry) (bool, error) {
		t, err := time.Parse(time.RFC3339Nano, string(msg.Value))
		if err != nil {
			return false, fmt.Errorf("converting string to timestamp: %s", err)
		}
		o := msg.Offset
		c := Consume{o, t}
		logger.Infof("Consumed message: %v", c)
		cons <- c

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
		logger.Errorf("unable to set up kafka consumer: %s", err)
		return
	}

	logger.Infof("Started message consumer.")

	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

	produce := func() {
		timer := time.Now()
		offset, err := prod.Produce(kafka.Message(timer.Format(time.RFC3339Nano)))
		ProduceLatency.Observe(time.Now().Sub(timer).Seconds())
		if err == nil {
			logger.Infof("Produced message: %v", Consume{
				offset:    offset,
				timeStamp: timer,
			})
			LastProducedTimestamp.SetToCurrentTime()
			LastProducedOffset.Set(float64(offset))
		} else {
			logger.Errorf("unable to produce canary message on Kafka: %s", err)
			if kafka.IsErrUnauthorized(err) {
				quit <- fmt.Errorf("credentials rotated or invalidated")
			}
		}
	}

	logger.Infof("Ready.")

	produceTicker := time.NewTicker(viper.GetDuration(MessageInterval))

	for {
		select {
		case <-produceTicker.C:
			produce()
		case msg := <-cons:
			LastConsumedTimestamp.SetToCurrentTime()
			ConsumeLatency.Observe(time.Now().Sub(msg.timeStamp).Seconds())
			LastConsumedOffset.Set(float64(msg.offset))
		case err := <-quit:
			logger.Errorf("quit: %s", err)
			os.Exit(ExitRuntime)
		case sig := <-signals:
			logger.Infof("exiting due to signal: %s", strings.ToUpper(sig.String()))
			os.Exit(ExitOK)
		}
	}
}
