package utils

import (
	"reflect"

	"github.com/aiven/aiven-go-client/v2"
)

func toMap(tags []aiven.KafkaTopicTag) map[string]string {
	mappedTags := make(map[string]string, 0)
	for _, tag := range tags {
		mappedTags[tag.Key] = tag.Value
	}
	return mappedTags
}

func topicReqComp(expected, actual interface{}, expectedTags, actualTags map[string]string) bool {
	delete(expectedTags, "touched-at")
	delete(actualTags, "touched-at")
	tagsEqual := reflect.DeepEqual(expectedTags, actualTags)
	requestEqual := reflect.DeepEqual(expected, actual)
	return tagsEqual && requestEqual
}

func TopicCreateReqComp(expected aiven.CreateKafkaTopicRequest) func(actual aiven.CreateKafkaTopicRequest) bool {
	return func(actual aiven.CreateKafkaTopicRequest) bool {
		expectedTags := toMap(expected.Tags)
		actualTags := toMap(actual.Tags)
		expected.Tags = nil
		actual.Tags = nil
		return topicReqComp(expected, actual, expectedTags, actualTags)
	}
}

func TopicUpdateReqComp(expected aiven.UpdateKafkaTopicRequest) func(actual aiven.UpdateKafkaTopicRequest) bool {
	return func(actual aiven.UpdateKafkaTopicRequest) bool {
		expectedTags := toMap(expected.Tags)
		actualTags := toMap(actual.Tags)
		expected.Tags = nil
		actual.Tags = nil
		return topicReqComp(expected, actual, expectedTags, actualTags)
	}
}
