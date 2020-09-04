package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	kafkaratormetrics "github.com/nais/kafkarator/pkg/metrics"
	"github.com/nais/kafkarator/pkg/metrics/clustercollector"
	"github.com/spf13/viper"
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

const (
	ExitOK = iota
	ExitController
	ExitAiven
	ExitReconciler
	ExitManager
	ExitConfig
)

const (
	AivenToken          = "aiven-token"
	MetricsAddress      = "metrics-address"
	TopicReportInterval = "topic-report-interval"
	LogFormat           = "log-format"

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

	aivenClient, err := aiven.NewTokenClient(viper.GetString(AivenToken), "")
	if err != nil {
		logger.Println(err)
		os.Exit(ExitAiven)
	}

	reconciler := &controllers.TopicReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Aiven:  aivenClient,
		Logger: logger,
	}

	if err = reconciler.SetupWithManager(mgr); err != nil {
		logger.Println(err)
		os.Exit(ExitReconciler)
	}

	logger.Info("Kafkarator running")

	collector := &clustercollector.Topic{
		Client:         mgr.GetClient(),
		Aiven:          aivenClient,
		Logger:         logger,
		ReportInterval: viper.GetDuration(TopicReportInterval),
	}
	go collector.Run()

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Println(err)
		os.Exit(ExitManager)
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
