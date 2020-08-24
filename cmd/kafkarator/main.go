package main

import (
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var scheme = runtime.NewScheme()

func main() {
}

func init() {
	err := clientgoscheme.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}
}
