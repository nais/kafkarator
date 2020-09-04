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
	retentionHours    = 15
)

func intp(i int) *int {
	return &i
}

func TestCreateTopic(t *testing.T) {
	client, err := aiven.NewTokenClient(os.Getenv("AIVEN_TOKEN"), "")
	if err != nil {
		panic(err)
	}

	req := aiven.CreateKafkaTopicRequest{
		Partitions:     intp(partitions),
		Replication:    intp(replication),
		RetentionHours: intp(retentionHours),
		TopicName:      topicName,
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

	topic, err := waitForTopic(time.Second*60, time.Second*5)
	assert.NoError(t, err)
	if err != nil {
		t.FailNow()
	}

	t.Logf("Topic state is %s", topic.State)
	assert.Equal(t, topicName, topic.TopicName)
	assert.Len(t, topic.Partitions, partitions)
	assert.Equal(t, replication, topic.Replication)
	assert.Equal(t, retentionHours, topic.RetentionHours)

	updatereq := aiven.UpdateKafkaTopicRequest{
		Partitions: intp(updatedPartitions),
	}

	t.Logf("Updating topic...")
	err = client.KafkaTopics.Update(project, service, topic.TopicName, updatereq)
	assert.NoError(t, err)
	if err == nil {
		t.Logf("Updating topic OK")
	}

	_, err = waitFor(time.Second*60, time.Second*5, func(topic *aiven.KafkaTopic) bool {
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
