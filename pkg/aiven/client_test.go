// +build integration

package aiven_test

import (
	"os"
	"testing"
	"time"

	"github.com/nais/kafkarator/pkg/aiven"
)

func TestClient_CreateTopic(t *testing.T) {
	client := &aiven.Client{
		Token: os.Getenv("AIVEN_TOKEN"),
	}
	project := "nav-integration"
	service := "integration-test-service"
	retention := time.Hour * 36
	payload := aiven.CreateTopicRequest{
		Config: aiven.Config{
			"retention_ms": retention.Milliseconds(),
		},
		TopicName:   "integration-test",
		Partitions:  1,
		Replication: 2,
	}
	err := client.CreateTopic(project, service, payload)
	if err != nil {
		t.Error(err)
		t.Fail()
	}
}
