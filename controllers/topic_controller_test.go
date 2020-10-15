package controllers_test

import (
	"testing"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/controllers"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConvertSecret(t *testing.T) {
	testSecret := controllers.SecretData{
		User: aiven.ServiceUser{
			Username:   "testapp-team1",
			AccessCert: "cert",
			AccessKey:  "key",
		},
		ResourceVersion: "testSecret-ResourceVersion",
		App:             "testSecret-App",
		Pool:            "testSecret-Pool",
		Name:            "testSecret-Name",
		Team:            "testSecret-Team",
		Brokers:         "testSecret-Brokers",
		Registry:        "testSecret-Registry",
		Ca:              "testSecret-Ca",
	}

	expectedSecret := v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      testSecret.Name,
			Namespace: testSecret.Team,
			Labels: map[string]string{
				"Team": testSecret.Team,
			},
			Annotations: map[string]string{
				"kafka.nais.io/Pool":        testSecret.Pool,
				"kafka.nais.io/application": testSecret.App,
			},
			ResourceVersion: testSecret.ResourceVersion,
		},
		StringData: map[string]string{
			controllers.KafkaCertificate:    testSecret.User.AccessCert,
			controllers.KafkaPrivateKey:     testSecret.User.AccessKey,
			controllers.KafkaBrokers:        testSecret.Brokers,
			controllers.KafkaSchemaRegistry: testSecret.Registry,
			controllers.KafkaCA:             testSecret.Ca,
		},
		Type: v1.SecretTypeOpaque,
	}

	convertedTestSecret := controllers.ConvertSecret(testSecret)

	assert.Equal(t, expectedSecret, convertedTestSecret)
}
