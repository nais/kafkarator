package manager

import (
	"strings"

	aiven "github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/pkg/metrics"
)

var _ AclAdapter = AivenSchemaRegistryACLAdapter{}

type SchemaRegistryAclInterface interface {
	List(project, serviceName string) ([]*aiven.KafkaSchemaRegistryACL, error)
	Create(project, service string, req aiven.CreateKafkaSchemaRegistryACLRequest) (*aiven.KafkaSchemaRegistryACL, error)
	Delete(project, service, aclID string) error
}
type AivenSchemaRegistryACLAdapter struct {
	SchemaRegistryAclInterface SchemaRegistryAclInterface
	Project                    string
	Service                    string
}

// Create implements AivenAdapter.
func (a AivenSchemaRegistryACLAdapter) Create(acl Acl) (Acl, error) {
	req := aiven.CreateKafkaSchemaRegistryACLRequest{
		Permission: acl.Permission,
		Resource:   "Subject:" + acl.TopicPattern,
		Username:   acl.Username,
	}
	aivenAcl, err := metrics.GObserveAivenLatency("SCHEMA_ACL_List", a.Project, func() (*aiven.KafkaSchemaRegistryACL, error) {
		return a.SchemaRegistryAclInterface.Create(a.Project, a.Service, req)
	})
	if err != nil {
		return Acl{}, err
	}

	return fromKafkaSchemaRegistryAcl(aivenAcl), nil
}

// Delete implements AivenAdapter.
func (a AivenSchemaRegistryACLAdapter) Delete(aclID string) error {
	_, err := metrics.GObserveAivenLatency("SCHEMA_ACL_Delete", a.Project, func() (any, error) {
		return nil, a.SchemaRegistryAclInterface.Delete(a.Project, a.Service, aclID)
	})
	return err
}

// List implements AivenAdapter.
func (a AivenSchemaRegistryACLAdapter) List() ([]Acl, error) {
	aivenAcls, err := metrics.GObserveAivenLatency("SCHEMA_ACL_List", a.Project, func() ([]*aiven.KafkaSchemaRegistryACL, error) {
		return a.SchemaRegistryAclInterface.List(a.Project, a.Service)
	})
	if err != nil {
		return nil, err
	}

	acls := make([]Acl, len(aivenAcls))
	for _, acl := range aivenAcls {
		acls = append(acls, fromKafkaSchemaRegistryAcl(acl))
	}

	return acls, nil
}

func fromKafkaSchemaRegistryAcl(aivenKafkaAcl *aiven.KafkaSchemaRegistryACL) Acl {
	return Acl{
		ID:           aivenKafkaAcl.ID,
		Permission:   aivenKafkaAcl.Permission,
		TopicPattern: strings.TrimPrefix(aivenKafkaAcl.Resource, "Subject:"),
		Username:     aivenKafkaAcl.Username,
	}
}
