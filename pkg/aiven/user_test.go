// +build integration

package aiven_test

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"

	"github.com/dgrijalva/jwt-go"
	"github.com/nais/kafkarator/pkg/aiven"
	"github.com/stretchr/testify/assert"
)

func TestClient_CreateUser(t *testing.T) {
	client := &aiven.Client{
		Token:   os.Getenv("AIVEN_TOKEN"),
		Project: "nav-integration",
		Service: "integration-test-service",
	}

	username := "integration-test-user"

	t.Logf("Creating user %s", username)

	user, err := client.CreateUser(username)
	assert.NoError(t, err)

	assert.Equal(t, username, user.Username)

	block, _ := pem.Decode([]byte(user.AccessCert))
	cert, err := x509.ParseCertificate(block.Bytes)
	assert.NoError(t, err)
	assert.NotNil(t, cert)
	assert.True(t, cert.BasicConstraintsValid)

	t.Logf("Access certificate of new user %s", username)
	t.Logf(user.AccessCert)

	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(user.AccessKey))
	assert.NoError(t, err)
	assert.NotNil(t, key)

	t.Logf("Certificate key of new user %s", username)
	t.Logf(user.AccessKey)

	t.Logf("Deleting user %s", username)

	err = client.DeleteUser(username)
	assert.NoError(t, err)
}
