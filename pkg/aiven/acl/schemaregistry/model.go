package schemaregistry

import (
	"fmt"

	"github.com/aiven/aiven-go-client"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
)

type Acl struct {
	ID         string
	Permission string
	Resource   string
	Username   string
}

type Acls []Acl

func FromTopicACL(resource string, topicAcl *kafka_nais_io_v1.TopicACL, namegen func(topicAcl *kafka_nais_io_v1.TopicACL) (string, error)) (Acl, error) {
	username, err := namegen(topicAcl)
	if err != nil {
		return Acl{}, err
	}
	return Acl{
		Permission: topicAcl.Access,
		Resource:   resource,
		Username:   username,
	}, nil
}

func FromKafkaSchemaRegistryACL(kafkaAcl *aiven.KafkaSchemaRegistryACL) Acl {
	return Acl{
		ID:         kafkaAcl.ID,
		Permission: kafkaAcl.Permission,
		Resource:   kafkaAcl.Resource,
		Username:   kafkaAcl.Username,
	}
}

func (a *Acls) Contains(other Acl) bool {
	for _, mine := range *a {
		if mine.Username == other.Username &&
			mine.Resource == other.Resource &&
			mine.Permission == other.Permission {
			return true
		}
	}
	return false
}

func (a Acl) String() string {
	return fmt.Sprintf("Acl{Username:'%s', Permission:'%s', Resource:'%s', ID:'%s'}",
		a.Username, a.Permission, a.Resource, a.ID)
}
