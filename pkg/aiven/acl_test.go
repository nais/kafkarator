// +build integration

package kafkarator_aiven_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/aiven/aiven-go-client"
	"github.com/stretchr/testify/assert"
)

func TestCreateACL(t *testing.T) {
	client, err := aiven.NewTokenClient(os.Getenv("AIVEN_TOKEN"), "")
	if err != nil {
		panic(err)
	}

	const topicName = "mytopic"
	req := aiven.CreateKafkaACLRequest{
		Permission: "write",
		Topic:      topicName,
		Username:   "test_user",
	}

	acl1, err := client.KafkaACLs.Create(project, service, req)
	assert.NoError(t, err)
	assert.NotNil(t, acl1)

	acl2, err := client.KafkaACLs.Create(project, service, req)
	assert.Error(t, err)
	assert.Nil(t, acl2)
	if err != nil {
		aivenError := err.(aiven.Error)
		assert.Equal(t, http.StatusConflict, aivenError.Status)
	}

	req.Permission = "read"
	acl3, err := client.KafkaACLs.Create(project, service, req)
	assert.NoError(t, err)
	assert.NotNil(t, acl3)

	err = client.KafkaACLs.Delete(project, service, acl1.ID)
	assert.NoError(t, err)
	err = client.KafkaACLs.Delete(project, service, acl3.ID)
	assert.NoError(t, err)
}
