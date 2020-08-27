package main

import (
	"os"

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
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})

	if err != nil {
		log.Println(err)
		os.Exit(ExitController)
	}

	aivenClient, err := aiven.NewTokenClient(os.Getenv("AIVEN_TOKEN"), "")
	if err != nil {
		log.Println(err)
		os.Exit(ExitAiven)
	}

	reconciler := &controllers.TopicReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Aiven:  aivenClient,
	}

	if err = reconciler.SetupWithManager(mgr); err != nil {
		log.Println(err)
		os.Exit(ExitReconciler)
	}

	log.Info("Kafkarator running")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Println(err)
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

	// +kubebuilder:scaffold:scheme
}
