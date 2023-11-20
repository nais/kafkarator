package manager

import (
	"fmt"
	"strings"

	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
)

type Acl struct {
	ID           string
	Permission   string
	TopicPattern string
	Username     string
}

func (a Acl) String() string {
	return fmt.Sprintf("%s %s %s %s", a.ID, a.Permission, a.TopicPattern, a.Username)
}

func relaxedCmpPermission(p1, p2 string) bool {
	if p1 == p2 {
		return true
	}

	if strings.HasPrefix(p1, "schema_registry") {
		return permissionFromTopic(p2) == p1
	} else if strings.HasPrefix(p2, "schema_registry") {
		return permissionFromTopic(p1) == p2
	}

	return false
}

func (a Acl) Equal(other Acl) bool {
	return relaxedCmpPermission(a.Permission, other.Permission) &&
		a.TopicPattern == other.TopicPattern &&
		a.Username == other.Username
}

func FromTopicACL(topic string, topicAcl *kafka_nais_io_v1.TopicACL, namegen func(topicAcl *kafka_nais_io_v1.TopicACL) (string, error)) (Acl, error) {
	username, err := namegen(topicAcl)
	if err != nil {
		return Acl{}, err
	}
	return Acl{
		Permission:   topicAcl.Access,
		TopicPattern: topic,
		Username:     username,
	}, nil
}
