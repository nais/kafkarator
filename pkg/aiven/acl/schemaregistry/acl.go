package schemaregistry

import (
	"fmt"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/pkg/metrics"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	log "github.com/sirupsen/logrus"
)

type Interface interface {
	List(project, serviceName string) ([]*aiven.KafkaSchemaRegistryACL, error)
	Create(project, service string, req aiven.CreateKafkaSchemaRegistryACLRequest) (*aiven.KafkaSchemaRegistryACL, error)
	Delete(project, service, aclID string) error
}

type Source interface {
	ResourceName() string
	Pool() string
	ACLs() kafka_nais_io_v1.TopicACLs
}

type Manager struct {
	AivenSchemaACLs Interface
	Project         string
	Service         string
	Source          Source
	Logger          log.FieldLogger
}

// Synchronize Syncs the ACL spec in the Source resource with Aiven.
//
//	Missing ACL definitions are created, unnecessary definitions are deleted.
func (r *Manager) Synchronize() error {
	existingAcls, err := r.getExistingAcls()
	if err != nil {
		return err
	}

	wantedAcls, err := r.getWantedAcls()
	if err != nil {
		return err
	}

	toAdd := NewACLs(existingAcls, wantedAcls)
	toDelete := DeleteACLs(existingAcls, wantedAcls)

	err = r.add(toAdd)
	if err != nil {
		return err
	}

	err = r.delete(toDelete)
	if err != nil {
		return err
	}

	return nil
}

func (r *Manager) getExistingAcls() ([]Acl, error) {
	var kafkaAcls []*aiven.KafkaSchemaRegistryACL
	err := metrics.ObserveAivenLatency("SCHEMA_ACL_List", r.Project, func() error {
		var err error
		kafkaAcls, err = r.AivenSchemaACLs.List(r.Project, r.Service)
		return err
	})
	if err != nil {
		return nil, err
	}

	acls := schemaACLs(kafkaAcls, r.Source.ResourceName())
	return acls, nil
}

func (r *Manager) getWantedAcls() ([]Acl, error) {
	topicAcls := r.Source.ACLs()

	wantedAcls := make([]Acl, 0, len(topicAcls))
	for _, aclSpec := range topicAcls {
		newNameAcl, err := FromTopicACL(r.Source.ResourceName(), &aclSpec, func(topicAcl *kafka_nais_io_v1.TopicACL) (string, error) {
			return topicAcl.ServiceUserNameWithSuffix("*")
		})
		if err != nil {
			return nil, err
		}
		wantedAcls = append(wantedAcls, newNameAcl)
	}
	return wantedAcls, nil
}

func (r *Manager) add(toAdd []Acl) error {
	for _, acl := range toAdd {
		req := aiven.CreateKafkaSchemaRegistryACLRequest{
			Permission: acl.Permission,
			Resource:   acl.Resource,
			Username:   acl.Username,
		}

		err := metrics.ObserveAivenLatency("SCHEMA_ACL_Create", r.Project, func() error {
			var err error
			_, err = r.AivenSchemaACLs.Create(r.Project, r.Service, req)
			return err
		})
		if err != nil {
			return err
		}

		r.Logger.WithFields(log.Fields{
			"acl_username":   req.Username,
			"acl_permission": req.Permission,
		}).Infof("Created ACL entry")
	}
	return nil
}

func (r *Manager) delete(toDelete []Acl) error {
	for _, acl := range toDelete {
		if len(acl.ID) == 0 {
			return fmt.Errorf("attemping to delete acl without ID: %v", acl)
		}
		err := metrics.ObserveAivenLatency("SCHEMA_ACL_Delete", r.Project, func() error {
			return r.AivenSchemaACLs.Delete(r.Project, r.Service, acl.ID)
		})
		if err != nil {
			return err
		}

		r.Logger.WithFields(log.Fields{
			"acl_id":         acl.ID,
			"acl_username":   acl.Username,
			"acl_permission": acl.Permission,
		}).Infof("Deleted ACL entry")
	}
	return nil
}

// NewACLs given a list of ACL specs, return a new list of ACL objects that does not already exist
func NewACLs(existingAcls, wantedAcls Acls) []Acl {
	candidates := make([]Acl, 0, len(wantedAcls))
	for _, wantedAcl := range wantedAcls {
		if !existingAcls.Contains(wantedAcl) {
			candidates = append(candidates, wantedAcl)
		}
	}
	return candidates
}

// DeleteACLs given a list of existing ACLs, return a new list of objects that don't exist in the cluster and should be deleted
func DeleteACLs(existingAcls, wantedAcls Acls) []Acl {
	candidates := make([]Acl, 0, len(existingAcls))
	for _, existingAcl := range existingAcls {
		if !wantedAcls.Contains(existingAcl) {
			candidates = append(candidates, existingAcl)
		}
	}
	return candidates
}

// filter out ACLs not matching the resource name
func schemaACLs(acls []*aiven.KafkaSchemaRegistryACL, resource string) []Acl {
	result := make([]Acl, 0, len(acls))
	for _, acl := range acls {
		if acl.Resource == resource {
			result = append(result, FromKafkaSchemaRegistryACL(acl))
		}
	}
	return result
}
