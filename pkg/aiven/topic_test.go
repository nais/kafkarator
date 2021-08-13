// +build integration

package kafkarator_aiven_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aiven/aiven-go-client"
	"github.com/stretchr/testify/assert"
)

const (
	project           = "nav-integration-test"
	service           = "nav-integration-test-kafka"
	topicName         = "integration-topic"
	partitions        = 1
	updatedPartitions = 2
	replication       = 2
	retentionHours    = 15 * time.Hour
	timeout           = 120 * time.Second
	retryInterval     = 5 * time.Second
)

func intp(i int) *int {
	return &i
}

func int64p(i int64) *int64 {
	return &i
}

func TestCreateTopic(t *testing.T) {
	token, result := os.LookupEnv("AIVEN_TOKEN")
	if !result {
		panic("You must set the AIVEN_TOKEN environment variable")
	}
	client, err := aiven.NewTokenClient(token, "")
	if err != nil {
		panic(err)
	}

	req := aiven.CreateKafkaTopicRequest{
		Partitions:  intp(partitions),
		Replication: intp(replication),
		TopicName:   topicName,
		Config: aiven.KafkaTopicConfig{
			RetentionMs: int64p(retentionHours.Milliseconds()),
		},
		Tags: nil,
	}

	t.Logf("Creating topic...")
	err = client.KafkaTopics.Create(project, service, req)

	assert.NoError(t, err)
	if err == nil {
		t.Logf("Creating topic OK")
	}

	waitFor := func(timeout, retry time.Duration, condition func(*aiven.KafkaTopic) bool) (*aiven.KafkaTopic, error) {
		timeoutTimer := time.NewTimer(timeout)
		retryTimer := time.NewTicker(retry)

		for {
			select {
			case <-timeoutTimer.C:
				return nil, fmt.Errorf("topic request not propagated in due time, aborting")

			case <-retryTimer.C:
				t.Logf("Attempting topic retrieval...")
				topic, err := client.KafkaTopics.Get(project, service, topicName)
				if err == nil {
					if condition(topic) {
						return topic, nil
					}
					t.Logf("Topic conditional check failed, retrying in 5 seconds...")
				} else {
					t.Logf("Topic not yet created, retrying in 5 seconds...")
				}
			}
		}
	}

	waitForTopic := func(timeout, retry time.Duration) (*aiven.KafkaTopic, error) {
		return waitFor(timeout, retry, func(topic *aiven.KafkaTopic) bool { return true })
	}

	topic, err := waitForTopic(timeout, retryInterval)
	assert.NoError(t, err)
	if err != nil {
		t.FailNow()
	}

	t.Logf("Topic state is %s", topic.State)
	assert.Equal(t, topicName, topic.TopicName)
	assert.Len(t, topic.Partitions, partitions)
	assert.Equal(t, replication, topic.Replication)
	assert.Equal(t, int(retentionHours.Hours()), *topic.RetentionHours)
	assert.Equal(t, retentionHours, time.Millisecond*time.Duration(topic.Config.RetentionMs.Value))

	updatereq := aiven.UpdateKafkaTopicRequest{
		Partitions: intp(updatedPartitions),
	}

	t.Logf("Updating topic...")
	err = client.KafkaTopics.Update(project, service, topic.TopicName, updatereq)
	assert.NoError(t, err)
	if err == nil {
		t.Logf("Updating topic OK")
	}

	_, err = waitFor(timeout, retryInterval, func(topic *aiven.KafkaTopic) bool {
		return len(topic.Partitions) == updatedPartitions
	})
	assert.NoError(t, err)

	t.Logf("Deleting topic...")

	err = client.KafkaTopics.Delete(project, service, topic.TopicName)
	assert.NoError(t, err)

	if err == nil {
		t.Logf("Deleting topic OK")
	}
}
