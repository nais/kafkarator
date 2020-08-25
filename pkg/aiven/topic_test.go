// +build integration

package aiven_test

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/nais/kafkarator/pkg/aiven"
	"github.com/stretchr/testify/assert"
)

func equalConfig(config aiven.Config, configResponse aiven.ConfigResponse, key string) error {
	switch v := configResponse[key].Value.(type) {
	case float64:
		// Aiven returns scientific notation in float for numbers (???)
		if int64(v) == config[key].(int64) {
			return nil
		}
	default:
		if reflect.DeepEqual(config[key], configResponse[key].Value) {
			return nil
		}
	}
	return fmt.Errorf("configuration option '%s' of type %T and value %v does not match expected value %v", key, configResponse[key].Value, config[key], configResponse[key].Value)
}

func TestClient_CreateTopic(t *testing.T) {
	client := &aiven.Client{
		Token:   os.Getenv("AIVEN_TOKEN"),
		Project: "nav-integration",
		Service: "integration-test-service",
	}
	topicName := "integration-test"
	retention := time.Hour * 36
	payload := aiven.CreateTopicRequest{
		Config: aiven.Config{
			"retention_ms": retention.Milliseconds(),
		},
		TopicName:   topicName,
		Partitions:  1,
		Replication: 2,
	}
	err := client.CreateTopic(payload)
	assert.NoError(t, err)

	t.Logf("Topic creation OK, proceeding with retrieval")

	createTopicWithTimeout := func(timeout, retry time.Duration) (*aiven.TopicResponse, error) {
		timeoutTimer := time.NewTimer(timeout)
		retryTimer := time.NewTicker(retry)

		for {
			select {
			case <-timeoutTimer.C:
				return nil, fmt.Errorf("topic creation not propagated in due time, aborting")

			case <-retryTimer.C:
				t.Logf("Attempting topic retrieval...")
				topic, err := client.GetTopic(topicName)
				if err == nil {
					return topic, nil
				}
				t.Logf("Topic not yet created, retrying in 5 seconds...")
			}
		}
	}

	topic, err := createTopicWithTimeout(time.Second*60, time.Second*5)
	assert.NoError(t, err)
	if err != nil {
		t.FailNow()
	}

	t.Logf("Topic retrieval OK, checking if equal")

	assert.Equal(t, payload.Partitions, len(topic.Partitions))
	assert.Equal(t, payload.Replication, topic.Replication)

	for k := range payload.Config {
		assert.NoError(t, equalConfig(payload.Config, topic.Config, k))
	}

	t.Logf("Topic looks equal to request, finalizing by deleting topic")

	err = client.DeleteTopic(topicName)

	assert.NoError(t, err)
}
