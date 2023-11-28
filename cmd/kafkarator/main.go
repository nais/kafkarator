package main

import (
	"context"
	"fmt"
	"github.com/nais/liberator/pkg/aiven/service"
	"os"
	"os/signal"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"strings"
	"syscall"
	"time"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/controllers"
	"github.com/nais/kafkarator/pkg/aiven"
	kafkaratormetrics "github.com/nais/kafkarator/pkg/metrics"
	"github.com/nais/kafkarator/pkg/metrics/collectors"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"github.com/nais/liberator/pkg/conftools"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
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

const (
	AivenToken               = "aiven-token"
	LogFormat                = "log-format"
	MetricsAddress           = "metrics-address"
	Projects                 = "projects"
	RequeueInterval          = "requeue-interval"
	SyncPeriod               = "sync-period"
	TopicReportInterval      = "topic-report-interval"
	SchemaRegistryACLEnabled = "schema-registry-acl-enabled"
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
	flag.String(MetricsAddress, "127.0.0.1:8080", "The address the metric endpoint binds to.")
	flag.String(LogFormat, "text", "Log format, either 'text' or 'json'")
	flag.Duration(TopicReportInterval, time.Minute*5, "The interval for topic metrics reporting")
	flag.Duration(RequeueInterval, time.Minute*5, "Requeueing interval when topic synchronization to Aiven fails")
	flag.Duration(SyncPeriod, time.Hour*1, "How often to re-synchronize all Topic resources including credential rotation")
	flag.Bool(SchemaRegistryACLEnabled, false, "Enable ACLs for Schema Registry")
	flag.StringSlice(Projects, []string{"nav-integration-test"}, "List of projects allowed to operate on")

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

	log.Infof("--- Current configuration ---")
	for _, cfg := range conftools.Format([]string{
		AivenToken,
	}) {
		log.Info(cfg)
	}
	log.Infof("--- End configuration ---")

	syncPeriod := viper.GetDuration(SyncPeriod)
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: viper.GetString(MetricsAddress),
		},
	})

	if err != nil {
		logger.Println(err)
		os.Exit(ExitController)
	}

	terminator, cancel := context.WithCancel(context.Background())
	logger.Info("Kafkarator running")

	go startReconcilers(quit, logger, mgr)

	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		for {
			select {
			case err := <-quit:
				logger.Errorf("terminating unexpectedly: %s", err)
				cancel()
				os.Exit(ExitRuntime)
			case sig := <-signals:
				logger.Infof("exiting due to signal: %s", strings.ToUpper(sig.String()))
				cancel()
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

func startReconcilers(quit QuitChannel, logger *log.Logger, mgr manager.Manager) {
	aivenClient, err := aiven.NewTokenClient(viper.GetString(AivenToken), "")
	if err != nil {
		quit <- fmt.Errorf("unable to set up aiven client: %s", err)
		return
	}

	nameResolver := service.NewCachedNameResolver(aivenClient.Services)
	schemaRegistryACLEnabled := viper.GetBool(SchemaRegistryACLEnabled)

	topicReconciler := &controllers.TopicReconciler{
		Aiven: kafkarator_aiven.Interfaces{
			KafkaAcls:          aivenClient.KafkaACLs,
			SchemaRegistryAcls: aivenClient.KafkaSchemaRegistryACLs,
			Topics:             aivenClient.KafkaTopics,
			NameResolver:       nameResolver,
		},
		SchemaRegistryACLEnabled: schemaRegistryACLEnabled,
		Client:                   mgr.GetClient(),
		Logger:                   logger,
		Projects:                 viper.GetStringSlice(Projects),
		RequeueInterval:          viper.GetDuration(RequeueInterval),
	}
	if err = topicReconciler.SetupWithManager(mgr); err != nil {
		quit <- fmt.Errorf("unable to set up topicReconciler: %s", err)
		return
	}

	streamReconciler := &controllers.StreamReconciler{
		Client: mgr.GetClient(),
		Aiven: kafkarator_aiven.Interfaces{
			KafkaAcls:          aivenClient.KafkaACLs,
			SchemaRegistryAcls: aivenClient.KafkaSchemaRegistryACLs,
			Topics:             aivenClient.KafkaTopics,
			NameResolver:       nameResolver,
		},
		SchemaRegistryACLEnabled: schemaRegistryACLEnabled,
		Logger:                   logger,
		Projects:                 viper.GetStringSlice(Projects),
		RequeueInterval:          viper.GetDuration(RequeueInterval),
	}
	if err = streamReconciler.SetupWithManager(mgr); err != nil {
		quit <- fmt.Errorf("unable to set up streamReconciler: %s", err)
		return
	}

	logger.Info("Reconcilers started")

	collectors.Start(&collectors.Opts{
		Client:         mgr.GetClient(),
		AivenClient:    aivenClient,
		ReportInterval: viper.GetDuration(TopicReportInterval),
		Projects:       viper.GetStringSlice(Projects),
		NameResolver:   nameResolver,
		Logger:         logger,
	})
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
