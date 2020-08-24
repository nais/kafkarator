package main

import (
	"fmt"
	"os"

	"github.com/nais/kafkarator/api/v1"
	"github.com/nais/kafkarator/controllers"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	// +kubebuilder:scaffold:imports
)

var scheme = runtime.NewScheme()

func main() {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})

	if err != nil {
		// TODO: Log error
		fmt.Println(err)
		os.Exit(1)
	}

	reconciler := &controllers.TopicReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	if err = reconciler.SetupWithManager(mgr); err != nil {
		// TODO: setupLog.Error(err, "unable to create controller", "controller", "Jwker")
		fmt.Println(err)
		os.Exit(2)
	}

	// TODO: setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		// TODO: setupLog.Error(err, "problem running manager")
		fmt.Println(err)
		os.Exit(3)
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
