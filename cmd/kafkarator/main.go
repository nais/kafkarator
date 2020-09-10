package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nais/kafkarator/pkg/kafka"
	"github.com/nais/kafkarator/pkg/kafka/consumer"
	"github.com/nais/kafkarator/pkg/kafka/producer"
	kafkaratormetrics "github.com/nais/kafkarator/pkg/metrics"
	"github.com/nais/kafkarator/pkg/metrics/clustercollector"
	"github.com/spf13/viper"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/api/v1"
	"github.com/nais/kafkarator/controllers"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	// +kubebuilder:scaffold:imports
)

var scheme = runtime.NewScheme()

type QuitChannel chan error

const (
	ExitOK = iota
	ExitController
	ExitAiven
	ExitReconciler
	ExitManager
	ExitConfig
	ExitRuntime
)

// Configuration options
const (
	AivenToken           = "aiven-token"
	PreSharedKey         = "psk"
	Follower             = "follower"
	KafkaBrokers         = "kafka-brokers"
	KafkaCAPath          = "kafka-ca-path"
	KafkaCertificatePath = "kafka-certificate-path"
	KafkaGroupID         = "kafka-group-id"
	KafkaKeyPath         = "kafka-key-path"
	KafkaTopic           = "kafka-topic"
	LogFormat            = "log-format"
	MetricsAddress       = "metrics-address"
	Primary              = "primary"
	TopicReportInterval  = "topic-report-interval"
)

const (
	LogFormatJSON = "json"
	LogFormatText = "text"
)

func init() {

	// Automatically read configuration options from environment variables.
	// i.e. --aiven-token will be configurable using KAFKARATOR_AIVEN_TOKEN.
	viper.SetEnvPrefix("KAFKARATOR")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	flag.String(AivenToken, "", "Administrator credentials for Aiven")
	flag.String(PreSharedKey, "", "Secret pre-shared key for encrypting and decrypting secrets sent over Kafka")
	flag.String(MetricsAddress, "127.0.0.1:8080", "The address the metric endpoint binds to.")
	flag.String(LogFormat, "text", "Log format, either 'text' or 'json'")
	flag.Duration(TopicReportInterval, time.Minute*5, "The interval for topic metrics reporting")
	flag.Bool(Primary, false, "If true, monitor kafka.nais.io/Topic resources and propagate them to Aiven and produce secrets")
	flag.Bool(Follower, false, "If true, consume secrets from Kafka topic and persist them to Kubernetes")

	// Kafka configuration
	hostname, _ := os.Hostname()
	flag.StringSlice(KafkaBrokers, []string{"localhost:9092"}, "Broker addresses for Kafka support")
	flag.String(KafkaTopic, "kafkarator-secrets", "Topic where Kafkarator cluster secrets are produced")
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
	quit := make(QuitChannel, 1)
	signals := make(chan os.Signal, 1)

	logger := log.New()
	logfmt, err := formatter(viper.GetString(LogFormat))
	if err != nil {
		logger.Error(err)
		os.Exit(ExitConfig)
	}

	logger.SetFormatter(logfmt)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: viper.GetString(MetricsAddress),
	})

	if err != nil {
		logger.Println(err)
		os.Exit(ExitController)
	}

	logger.Info("Kafkarator running")

	if viper.GetBool(Primary) {
		go primary(quit, logger, mgr)
	}

	if viper.GetBool(Follower) {
		go follower(quit, logger, mgr.GetClient())
	}

	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

	for {
		select {
		case err := <-quit:
			logger.Errorf("terminating unexpectedly: %s", err)
			os.Exit(ExitRuntime)
		case sig := <-signals:
			logger.Infof("exiting due to signal: %s", strings.ToUpper(sig.String()))
			os.Exit(ExitOK)
		}
	}
}

func tlsFromFiles() (cert, key, ca []byte, err error) {
	certPath := viper.GetString(KafkaCertificatePath)
	keyPath := viper.GetString(KafkaKeyPath)
	caPath := viper.GetString(KafkaCAPath)

	cert, err = ioutil.ReadFile(certPath)
	if err != nil {
		err = fmt.Errorf("unable to read certificate file %s: %s", certPath, err)
		return
	}

	key, err = ioutil.ReadFile(keyPath)
	if err != nil {
		err = fmt.Errorf("unable to read key file %s: %s", keyPath, err)
		return
	}

	ca, err = ioutil.ReadFile(caPath)
	if err != nil {
		err = fmt.Errorf("unable to read CA certificate file %s: %s", caPath, err)
		return
	}

	return
}

func primary(quit QuitChannel, logger *log.Logger, mgr manager.Manager) {

	aivenClient, err := aiven.NewTokenClient(viper.GetString(AivenToken), "")
	if err != nil {
		quit <- fmt.Errorf("unable to set up aiven client: %s", err)
		return
	}

	cert, key, ca, err := tlsFromFiles()
	if err != nil {
		quit <- fmt.Errorf("unable to set up TLS config: %s", err)
		return
	}

	tlsConfig, err := kafka.TLSConfig(cert, key, ca)
	if err != nil {
		quit <- fmt.Errorf("unable to set up TLS config: %s", err)
		return
	}

	prod, err := producer.New(viper.GetStringSlice(KafkaBrokers), viper.GetString(KafkaTopic), tlsConfig, logger)
	if err != nil {
		quit <- fmt.Errorf("unable to set up kafka producer: %s", err)
		return
	}

	reconciler := &controllers.TopicReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Aiven:    aivenClient,
		Producer: prod,
		Logger:   logger,
	}

	if err = reconciler.SetupWithManager(mgr); err != nil {
		quit <- fmt.Errorf("unable to set up reconciler: %s", err)
		return
	}

	logger.Info("Primary started")

	collector := &clustercollector.Topic{
		Client:         mgr.GetClient(),
		Aiven:          aivenClient,
		Logger:         logger,
		ReportInterval: viper.GetDuration(TopicReportInterval),
	}
	go collector.Run()

	terminator := make(chan struct{}, 1)

	if err := mgr.Start(terminator); err != nil {
		quit <- fmt.Errorf("manager stopped unexpectedly: %s", err)
		return
	}

	quit <- fmt.Errorf("manager has stopped")
}

func follower(quit QuitChannel, logger *log.Logger, client client.Client) {
	logger.Info("Follower started")

	cert, key, ca, err := tlsFromFiles()
	if err != nil {
		quit <- fmt.Errorf("unable to set up TLS config: %s", err)
		return
	}

	tlsConfig, err := kafka.TLSConfig(cert, key, ca)
	if err != nil {
		quit <- fmt.Errorf("unable to set up TLS config: %s", err)
		return
	}

	cons, err := consumer.New(viper.GetStringSlice(KafkaBrokers), viper.GetString(KafkaTopic), viper.GetString(KafkaGroupID), tlsConfig, logger)
	if err != nil {
		quit <- fmt.Errorf("unable to set up kafka consumer: %s", err)
		return
	}

	for msg := range cons.Messages {
		log.Infof(string(msg))
	}
}

func init() {
	err := clientgoscheme.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}

	err = kafka_nais_io_v1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}

	metrics.Registry.MustRegister(
		kafkaratormetrics.Topics,
		kafkaratormetrics.TopicsProcessed,
		kafkaratormetrics.ServiceUsers,
		kafkaratormetrics.AivenLatency,
		kafkaratormetrics.Acls,
	)
	// +kubebuilder:scaffold:scheme
}
