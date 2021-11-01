package utils

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const Finalizer = "kafkarator.kafka.nais.io"

func AppendFinalizer(in client.Object) {
	finalizers := in.GetFinalizers()
	if finalizers == nil {
		finalizers = make([]string, 0)
	}
	for _, v := range finalizers {
		if v == Finalizer {
			return
		}
	}
	finalizers = append(finalizers, Finalizer)
	in.SetFinalizers(finalizers)
}

func RemoveFinalizer(in client.Object) {
	finalizers := make([]string, 0, len(in.GetFinalizers()))
	for _, v := range in.GetFinalizers() {
		if v != Finalizer {
			finalizers = append(finalizers, v)
		}
	}
	in.SetFinalizers(finalizers)
}
