package manager

import (
	"fmt"

	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
)

type Acl struct {
	ID           string
	Permission   string
	TopicPattern string
	Username     string
}
type Acls []*Acl

func (a Acl) String() string {
	return fmt.Sprintf("%s %s %s %s", a.ID, a.Permission, a.TopicPattern, a.Username)
}
func (a Acl) Equal(other Acl) bool {
	return a.Permission == other.Permission &&
		a.TopicPattern == other.TopicPattern &&
		a.Username == other.Username

}
func (a *Acls) Contains(other Acl) bool {
	for _, mine := range *a {
		if mine.Username == other.Username &&
			mine.TopicPattern == other.TopicPattern &&
			mine.Permission == other.Permission {
			return true
		}
	}
	return false
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
