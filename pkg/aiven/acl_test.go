// +build integration

package aiven_test

import (
	"os"
	"testing"

	"github.com/nais/kafkarator/pkg/aiven"
	"github.com/stretchr/testify/assert"
)

func TestClient_CreateACL(t *testing.T) {
	client := &aiven.Client{
		Token:   os.Getenv("AIVEN_TOKEN"),
		Project: "nav-integration-test",
		Service: "nav-integration-test-kafka",
	}

	username := "integration-test-user"
	permission := "read"
	topicName := "integration-test"

	t.Logf("Creating ACL: %s can %s on %s", username, permission, topicName)

	acls, err := client.CreateACL(username, topicName, permission)
	assert.NoError(t, err)
	assert.Len(t, acls, 1)
	acl := acls[0]

	assert.Equal(t, permission, acl.Permission)
	assert.Equal(t, username, acl.Username)
	assert.Equal(t, topicName, acl.Topic)
	assert.NotEmpty(t, acl.ID)

	t.Logf("Deleting ACL with id %s", acl.ID)

	err = client.DeleteACL(acl.ID)
	assert.NoError(t, err)
}
