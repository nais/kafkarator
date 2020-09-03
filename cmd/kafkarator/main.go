package main

import (
	kafkaratormetrics "github.com/nais/kafkarator/pkg/metrics"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"time"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/api/v1"
	"github.com/nais/kafkarator/controllers"
	log "github.com/sirupsen/logrus"
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
)

func main() {
	logfmt := &log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339Nano,
	}
	logger := log.New()
	logger.SetFormatter(logfmt)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "127.0.0.1:8080",
	})

	if err != nil {
		logger.Println(err)
		os.Exit(ExitController)
	}

	aivenClient, err := aiven.NewTokenClient(os.Getenv("AIVEN_TOKEN"), "")
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
