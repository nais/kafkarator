package acl

import (
	"github.com/aiven/aiven-go-client"
	kafka_nais_io_v1 "github.com/nais/kafkarator/api/v1"
)

func aivenService(aivenProject string) string {
	return aivenProject + "-kafka"
}

func Update(aiven *aiven.Client, topic kafka_nais_io_v1.Topic) error {
	acls, err := aiven.KafkaACLs.List(topic.Spec.Pool, aivenService(topic.Spec.Pool))
	if err != nil {
		return err
	}
}
