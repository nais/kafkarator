package acl

import (
	"fmt"

	"github.com/aiven/aiven-go-client/v2"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
)

type Acl struct {
	ID         string
	Permission string
	Topic      string
	Username   string

	NativeIDs []string
}

type Acls []Acl

func FromTopicACL(topic string, topicAcl *kafka_nais_io_v1.TopicACL, namegen func(topicAcl *kafka_nais_io_v1.TopicACL) (string, error)) (Acl, error) {
	username, err := namegen(topicAcl)
	if err != nil {
		return Acl{}, err
	}
	return Acl{
		Permission: topicAcl.Access,
		Topic:      topic,
		Username:   username,
	}, nil
}

func FromKafkaACL(kafkaAcl *aiven.KafkaACL) Acl {
	return Acl{
		ID:         kafkaAcl.ID,
		Permission: kafkaAcl.Permission,
		Topic:      kafkaAcl.Topic,
		Username:   kafkaAcl.Username,
	}
}

func (a *Acls) Contains(other Acl) bool {
	for _, mine := range *a {
		if mine.Username == other.Username &&
			mine.Topic == other.Topic &&
			mine.Permission == other.Permission {
			return true
		}
	}
	return false
}

func (a Acl) String() string {
	return fmt.Sprintf(
		"Acl{Username:'%s', Permission:'%s', Topic:'%s', ID:'%s', NativeIDs:%v}",
		a.Username, a.Permission, a.Topic, a.ID, a.NativeIDs,
	)
}

type CreateKafkaACLRequest struct {
	Permission string `json:"permission"`
	Topic      string `json:"topic"`
	Username   string `json:"username"`
}
