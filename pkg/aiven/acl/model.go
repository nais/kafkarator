package acl

import (
	"fmt"

	"github.com/aiven/aiven-go-client/v2"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
)

type Acl struct {
	ID              string
	Permission      string
	Topic           string
	Username        string
	ResourcePattern string
}

type Acls []Acl

// ExistingAcl is the *observed* ACL (from Aiven), including the native IDs that must be deleted.
type ExistingAcl struct {
	Acl Acl
	IDs []string
}

type ExistingAcls []ExistingAcl

func FromTopicACL(topic string, topicAcl *kafka_nais_io_v1.TopicACL, resourcePattern string, namegen func(topicAcl *kafka_nais_io_v1.TopicACL) (string, error)) (Acl, error) {
	username, err := namegen(topicAcl)
	if err != nil {
		return Acl{}, err
	}
	return Acl{
		Permission:      topicAcl.Access,
		Topic:           topic,
		Username:        username,
		ResourcePattern: resourcePattern,
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

func (e *ExistingAcls) Contains(other Acl) bool {
	for _, mine := range *e {
		if mine.Acl.Username == other.Username &&
			mine.Acl.Topic == other.Topic &&
			mine.Acl.Permission == other.Permission {
			return true
		}
	}
	return false
}

func (a Acl) String() string {
	return fmt.Sprintf("Acl{Username:'%s', Permission:'%s', Topic:'%s', ResourcePattern:'%s'}",
		a.Username, a.Permission, a.Topic, a.ResourcePattern)
}

func (e ExistingAcl) String() string {
	return fmt.Sprintf("ExistingAcl{Acl:%s, IDs:%v}", e.Acl.String(), e.IDs)
}

type CreateKafkaACLRequest struct {
	Permission      string `json:"permission"`
	Topic           string `json:"topic"`
	Username        string `json:"username"`
	ResourcePattern string `json:"resource_pattern,omitempty"`
}
