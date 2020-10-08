// +build integration

package certificate_test

import (
	"encoding/base64"
	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/pkg/certificate"
	log "github.com/sirupsen/logrus"
	"os"
	"testing"
)

const (
	project  = "nav-integration-test"
	service  = "nav-integration-test-kafka"
	username = "test_user"
)

func TestCredStoreGenerator(t *testing.T) {
	log.Error("Starting test")
	client, err := aiven.NewTokenClient(os.Getenv("AIVEN_TOKEN"), "")
	if err != nil {
		log.Errorf("failed to create client: %v", err)
		panic(err)
	}

	test_user, err := client.ServiceUsers.Get(project, service, username)
	if err != nil {
		log.Errorf("failed to get service user: %v", err)
		req := aiven.CreateServiceUserRequest{Username: username}
		test_user, err = client.ServiceUsers.Create(project, service, req)
		if err != nil {
			log.Errorf("failed to create service user: %v", err)
			panic(err)
		}
	}

	caCert, err := client.CA.Get(project)
	if err != nil {
		log.Errorf("failed to get CA cert: %v", err)
		panic(err)
	}

	generator := certificate.NewExecGenerator(log.New())

	stores, err := generator.MakeCredStores(test_user.AccessKey, test_user.AccessCert, caCert)
	if err != nil {
		log.Errorf("failed to create cred stores: %v", err)
		panic(err)
	}

	log.Infof("Generated CredStores:")
	log.Infof("Secret: '%v'", stores.Secret)
	log.Info("Truststore:")
	log.Info(base64.StdEncoding.EncodeToString(stores.Truststore))
	log.Info("Keystore:")
	log.Info(base64.StdEncoding.EncodeToString(stores.Keystore))
}
