package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Shopify/sarama"
	"github.com/nais/kafkarator/api/v1"
	"github.com/nais/kafkarator/controllers"
	"github.com/nais/kafkarator/pkg/aiven"
	"github.com/nais/kafkarator/pkg/certificate"
	"github.com/nais/kafkarator/pkg/crypto"
	"github.com/nais/kafkarator/pkg/kafka"
	"github.com/nais/kafkarator/pkg/kafka/consumer"
	"github.com/nais/kafkarator/pkg/kafka/producer"
	kafkaratormetrics "github.com/nais/kafkarator/pkg/metrics"
	"github.com/nais/kafkarator/pkg/metrics/clustercollector"
	"github.com/nais/kafkarator/pkg/secretsync"
	"github.com/nais/kafkarator/pkg/utils"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	// +kubebuilder:scaffold:imports
)

var scheme = runtime.NewScheme()

type QuitChannel chan error

const (
	ExitOK = iota
	ExitController
	ExitConfig
	ExitRuntime
)

// Configuration options
const (
	AivenToken                   = "aiven-token"
	CredentialsLifetime          = "credentials-lifetime"
	Follower                     = "follower"
	KafkaBrokers                 = "kafka-brokers"
	KafkaCAPath                  = "kafka-ca-path"
	KafkaCertificatePath         = "kafka-certificate-path"
	KafkaGroupID                 = "kafka-group-id"
	KafkaKeyPath                 = "kafka-key-path"
	KafkaTopic                   = "kafka-topic"
	KubernetesWriteRetryInterval = "kubernetes-write-retry-interval"
	LogFormat                    = "log-format"
	MetricsAddress               = "metrics-address"
	PreSharedKey                 = "psk"
	Primary                      = "primary"
	Projects                     = "projects"
	SecretWriteTimeout           = "secret-write-timeout"
	TopicReportInterval          = "topic-report-interval"
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
	flag.Duration(CredentialsLifetime, time.Hour*24, "Maximum allowed lifetime of Kafka service user credentials")
	flag.String(PreSharedKey, "", "Secret pre-shared key for encrypting and decrypting secrets sent over Kafka")
	flag.String(MetricsAddress, "127.0.0.1:8080", "The address the metric endpoint binds to.")
	flag.String(LogFormat, "text", "Log format, either 'text' or 'json'")
	flag.Duration(TopicReportInterval, time.Minute*5, "The interval for topic metrics reporting")
	flag.Duration(SecretWriteTimeout, time.Second*2, "How much time to allocate for writing one secret to the cluster")
	flag.Duration(KubernetesWriteRetryInterval, time.Second*10, "Requeueing interval when Kubernetes writes fail")
	flag.Bool(Primary, false, "If true, monitor kafka.nais.io/Topic resources and propagate them to Aiven and produce secrets")
	flag.Bool(Follower, false, "If true, consume secrets from Kafka topic and persist them to Kubernetes")
	flag.StringSlice(Projects, []string{"nav-integration-test"}, "List of projects allowed to operate on")

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

	key, err := crypto.KeyFromHexString(viper.GetString(PreSharedKey))
	if err != nil {
		logger.Error(err)
		os.Exit(ExitConfig)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: viper.GetString(MetricsAddress),
	})

	if err != nil {
		logger.Println(err)
		os.Exit(ExitController)
	}

	cryptManager := crypto.NewManager(key)

	terminator := make(chan struct{}, 1)

	logger.Info("Kafkarator running")

	if viper.GetBool(Primary) {
		go primary(quit, logger, mgr, cryptManager)
	}

	if viper.GetBool(Follower) {
		go follower(quit, logger, mgr.GetClient(), cryptManager)
	}

	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

	go func() {
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
	}()

	if err := mgr.Start(terminator); err != nil {
		quit <- fmt.Errorf("manager stopped unexpectedly: %s", err)
		return
	}

	quit <- fmt.Errorf("manager has stopped")
}

func primary(quit QuitChannel, logger *log.Logger, mgr manager.Manager, cryptManager crypto.Manager) {

	aivenClient, err := aiven.NewTokenClient(viper.GetString(AivenToken), "")
	if err != nil {
		quit <- fmt.Errorf("unable to set up aiven client: %s", err)
		return
	}

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

	prod, err := producer.New(viper.GetStringSlice(KafkaBrokers), viper.GetString(KafkaTopic), tlsConfig, logger)
	if err != nil {
		quit <- fmt.Errorf("unable to set up kafka producer: %s", err)
		return
	}

	reconciler := &controllers.TopicReconciler{
		Aiven: kafkarator_aiven.Interfaces{
			ACLs:         aivenClient.KafkaACLs,
			CA:           aivenClient.CA,
			ServiceUsers: aivenClient.ServiceUsers,
			Service:      aivenClient.Services,
			Topics:       aivenClient.KafkaTopics,
		},
		Client:              mgr.GetClient(),
		CredentialsLifetime: viper.GetDuration(CredentialsLifetime),
		CryptManager:        cryptManager,
		Logger:              logger,
		Producer:            prod,
		Projects:            viper.GetStringSlice(Projects),
		RequeueInterval:     viper.GetDuration(KubernetesWriteRetryInterval),
		StoreGenerator:      certificate.NewExecGenerator(logger),
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
}

func follower(quit QuitChannel, logger *log.Logger, client client.Client, cryptManager crypto.Manager) {
	logger.Info("Follower started")

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

	secretsyncer := &secretsync.Synchronizer{
		Client:  client,
		Timeout: viper.GetDuration(SecretWriteTimeout),
	}

	callback := func(msg *sarama.ConsumerMessage, logger *log.Entry) (bool, error) {
		logger.Infof("Incoming message from Kafka")

		plaintext, err := cryptManager.Decrypt(msg.Value)
		if err != nil {
			return false, fmt.Errorf("decryption error in received message: %s", err)
		}

		secret := &v1.Secret{}
		err = secret.Unmarshal(plaintext)
		if err != nil {
			return false, fmt.Errorf("unmarshal error in received secret: %s", err)
		}

		logger = logger.WithFields(log.Fields{
			"secret_namespace": secret.Namespace,
			"secret_name":      secret.Name,
		})

		err = secretsyncer.Write(secret, logger)
		if err != nil {
			return true, fmt.Errorf("retriable error in persisting secret: %s", err)
		}

		logger.Infof("Successfully synchronized secret")

		return false, nil
	}

	_, err = consumer.New(consumer.Config{
		Brokers:           viper.GetStringSlice(KafkaBrokers),
		GroupID:           viper.GetString(KafkaGroupID),
		MaxProcessingTime: viper.GetDuration(SecretWriteTimeout),
		RetryInterval:     viper.GetDuration(KubernetesWriteRetryInterval),
		Topic:             viper.GetString(KafkaTopic),
		Callback:          callback,
		Logger:            logger,
		TlsConfig:         tlsConfig,
	})

	if err != nil {
		quit <- fmt.Errorf("unable to set up kafka consumer: %s", err)
	}

	return
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

	kafkaratormetrics.Register(metrics.Registry)
	// +kubebuilder:scaffold:scheme
}
