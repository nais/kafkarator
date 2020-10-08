package controllers

import (
	"testing"

	"github.com/aiven/aiven-go-client"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConvertSecret(t *testing.T) {
	testSecret := secretData{
		user: aiven.ServiceUser{
			Username:   "testapp-team1",
			AccessCert: "cert",
			AccessKey:  "key",
		},
		resourceVersion: "testSecret-resourceVersion",
		app:             "testSecret-app",
		pool:            "testSecret-pool",
		name:            "testSecret-name",
		team:            "testSecret-team",
		brokers:         "testSecret-brokers",
		registry:        "testSecret-registry",
		ca:              "testSecret-ca",
	}

	expectedSecret := v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      testSecret.name,
			Namespace: testSecret.team,
			Labels: map[string]string{
				"team": testSecret.team,
			},
			Annotations: map[string]string{
				"kafka.nais.io/pool":        testSecret.pool,
				"kafka.nais.io/application": testSecret.app,
			},
			ResourceVersion: testSecret.resourceVersion,
		},
		StringData: map[string]string{
			KafkaCertificate:    testSecret.user.AccessCert,
			KafkaPrivateKey:     testSecret.user.AccessKey,
			KafkaBrokers:        testSecret.brokers,
			KafkaSchemaRegistry: testSecret.registry,
			KafkaCA:             testSecret.ca,
		},
		Type: v1.SecretTypeOpaque,
	}

	convertedTestSecret := ConvertSecret(testSecret)

	assert.NotEmpty(t, convertedTestSecret)
	assert.NotZero(t, convertedTestSecret)
	assert.NotEqual(t, testSecret, convertedTestSecret)
	assert.Equal(t, expectedSecret, convertedTestSecret)
	assert.EqualValues(t, expectedSecret, convertedTestSecret)
}
