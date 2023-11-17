package manager

import (
	aiven "github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/pkg/metrics"
)

var _ AclAdapter = AivenKafkaAclAdapter{}

type KafkaAclInterface interface {
	List(project, serviceName string) ([]*aiven.KafkaACL, error)
	Create(project, service string, req aiven.CreateKafkaACLRequest) (*aiven.KafkaACL, error)
	Delete(project, service, aclID string) error
}

type AivenKafkaAclAdapter struct {
	KafkaAclInterface KafkaAclInterface
	Project           string
	Service           string
}

// Create implements AivenAdapter.
func (a AivenKafkaAclAdapter) Create(project string, service string, acl Acl) (Acl, error) {
	req := aiven.CreateKafkaACLRequest{
		Permission: acl.Permission,
		Topic:      acl.TopicPattern,
		Username:   acl.Username,
	}
	aivenAcl, err := metrics.GObserveAivenLatency("SCHEMA_ACL_List", a.Project, func() (*aiven.KafkaACL, error) {
		return a.KafkaAclInterface.Create(a.Project, a.Service, req)
	})
	if err != nil {
		return Acl{}, err
	}

	return fromKafkaAcl(aivenAcl), nil

}

// Delete implements AivenAdapter.
func (a AivenKafkaAclAdapter) Delete(project string, service string, aclID string) error {
	_, err := metrics.GObserveAivenLatency("SCHEMA_ACL_Delete", a.Project, func() (any, error) {
		return nil, a.KafkaAclInterface.Delete(a.Project, a.Service, aclID)
	})
	return err
}

// List implements AivenAdapter.
func (a AivenKafkaAclAdapter) List(project string, service string) ([]Acl, error) {
	aivenAcls, err := metrics.GObserveAivenLatency("SCHEMA_ACL_List", a.Project, func() ([]*aiven.KafkaACL, error) {
		return a.KafkaAclInterface.List(a.Project, a.Service)
	})
	if err != nil {
		return nil, err
	}

	acls := make([]Acl, len(aivenAcls))
	for _, acl := range aivenAcls {
		acls = append(acls, fromKafkaAcl(acl))
	}

	return acls, nil
}

func fromKafkaAcl(aivenKafkaAcl *aiven.KafkaACL) Acl {
	return Acl{
		ID:           aivenKafkaAcl.ID,
		Permission:   aivenKafkaAcl.Permission,
		TopicPattern: aivenKafkaAcl.Topic,
		Username:     aivenKafkaAcl.Username,
	}
}
