package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aiven/aiven-go-client/v2"
	generated_client "github.com/aiven/go-client-codegen"
	"github.com/go-logr/logr"
	"github.com/nais/kafkarator/controllers"
	"github.com/nais/kafkarator/pkg/aiven"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/kafkarator/pkg/aiven/adapter/aivengoclient"
	"github.com/nais/kafkarator/pkg/aiven/adapter/goclientcodegen"
	"github.com/nais/kafkarator/pkg/aiven/adapter/kafkanativeaclclient"
	kafkaratormetrics "github.com/nais/kafkarator/pkg/metrics"
	"github.com/nais/kafkarator/pkg/metrics/collectors"
	"github.com/nais/liberator/pkg/aiven/service"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"github.com/nais/liberator/pkg/conftools"
	"github.com/nais/liberator/pkg/logrus2logr"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	ctrl_client "sigs.k8s.io/controller-runtime/pkg/client"
	ctrl_log "sigs.k8s.io/controller-runtime/pkg/log"
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
	AivenToken          = "aiven-token"
	LogFormat           = "log-format"
	MetricsAddress      = "metrics-address"
	Projects            = "projects"
	RequeueInterval     = "requeue-interval"
	SyncPeriod          = "sync-period"
	TopicReportInterval = "topic-report-interval"
	DryRun              = "dry-run"
	LogLevel            = "log-level"
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
	flag.StringSlice(Projects, []string{"dev-nais-dev"}, "List of projects allowed to operate on")
	flag.Bool(DryRun, false, "If true, do not make any changes")
	flag.String(LogLevel, "info", "Log level, one of debug, info, warn, error")

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

	featureFlags, err := GetFeatureFlags()
	if err != nil {
		logger.Error(err)
		os.Exit(ExitConfig)
	}

	loglevel, err := log.ParseLevel(viper.GetString(LogLevel))
	if err != nil {
		logger.Errorf("invalid log level: %s", err)
		os.Exit(ExitConfig)
	}

	logger.SetFormatter(logfmt)
	logger.SetLevel(loglevel)

	logger.Infof("--- Current configuration ---")
	for _, cfg := range conftools.Format([]string{
		AivenToken,
	}) {
		logger.Info(cfg)
	}

	logger.Infof("--- Feature flags ---")
	featureFlags.Log(logger)

	logger.Infof("--- End configuration ---")

	logrSink := (&logrus2logr.Logrus2Logr{Logger: logger}).WithName("controller-runtime")
	ctrl_log.SetLogger(logr.New(logrSink))
	syncPeriod := viper.GetDuration(SyncPeriod)
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
		Client: ctrl_client.Options{
			DryRun: ptr.To(viper.GetBool(DryRun)),
		},
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: viper.GetString(MetricsAddress),
		},
		Logger: logr.New(logrSink.WithName("manager")),
	})

	if err != nil {
		logger.Println(err)
		os.Exit(ExitController)
	}

	terminator, cancel := context.WithCancel(context.Background())
	logger.Info("Kafkarator running")

	go startReconcilers(quit, logger, featureFlags, mgr)

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

func startReconcilers(quit QuitChannel, logger *log.Logger, featureFlags *FeatureFlags, mgr manager.Manager) {
	aivenClient, err := aiven.NewTokenClient(viper.GetString(AivenToken), "")
	if err != nil {
		quit <- fmt.Errorf("unable to set up aiven client: %s", err)
		return
	}

	var aclClient acl.Interface
	if featureFlags.GeneratedClient {
		generatedClient, err := generated_client.NewClient(generated_client.TokenOpt(viper.GetString(AivenToken)))
		if err != nil {
			quit <- fmt.Errorf("unable to set up aiven client: %s", err)
			return
		}
		if featureFlags.NativeKafkaAcl {
			logger.Info("Using native Kafka ACLs")
			aclClient = &kafkanativeaclclient.AclClient{
				Client: generatedClient,
			}
		} else {
			aclClient = &goclientcodegen.AclClient{
				Client: generatedClient,
			}
		}
	} else {
		aclClient = &aivengoclient.AclClient{
			KafkaACLHandler: aivenClient.KafkaACLs,
		}
	}

	nameResolver := service.NewCachedNameResolver(aivenClient.Services)

	topicReconciler := &controllers.TopicReconciler{
		Aiven: kafkarator_aiven.Interfaces{
			ACLs:         aclClient,
			Topics:       aivenClient.KafkaTopics,
			NameResolver: nameResolver,
		},
		Client:          mgr.GetClient(),
		Logger:          logger,
		Projects:        viper.GetStringSlice(Projects),
		RequeueInterval: viper.GetDuration(RequeueInterval),
		DryRun:          viper.GetBool(DryRun),
	}
	if err = topicReconciler.SetupWithManager(mgr); err != nil {
		quit <- fmt.Errorf("unable to set up topicReconciler: %s", err)
		return
	}

	streamReconciler := &controllers.StreamReconciler{
		Client: mgr.GetClient(),
		Aiven: kafkarator_aiven.Interfaces{
			ACLs:         aclClient,
			Topics:       aivenClient.KafkaTopics,
			NameResolver: nameResolver,
		},
		Logger:          logger,
		Projects:        viper.GetStringSlice(Projects),
		RequeueInterval: viper.GetDuration(RequeueInterval),
		DryRun:          viper.GetBool(DryRun),
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
