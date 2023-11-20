package manager

import (
	"fmt"

	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	log "github.com/sirupsen/logrus"
)

type AclAdapter interface {
	List() ([]Acl, error)
	Create(acl Acl) (Acl, error)
	Delete(aclID string) error
}

type Source interface {
	TopicName() string
	Pool() string
	ACLs() kafka_nais_io_v1.TopicACLs
}

type Manager struct {
	AivenAdapter AclAdapter
	Source       Source
	Logger       log.FieldLogger
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
	kafkaAcls, err := r.AivenAdapter.List()
	if err != nil {
		return nil, err
	}

	acls := filterACLs(kafkaAcls, r.Source.TopicName())
	return acls, nil
}

func (r *Manager) getWantedAcls() ([]Acl, error) {
	topicAcls := r.Source.ACLs()

	wantedAcls := make([]Acl, 0, len(topicAcls))
	for _, aclSpec := range topicAcls {
		newNameAcl, err := FromTopicACL(r.Source.TopicName(), &aclSpec, func(topicAcl *kafka_nais_io_v1.TopicACL) (string, error) {
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
		_, err := r.AivenAdapter.Create(acl)
		if err != nil {
			return err
		}

		r.Logger.WithFields(log.Fields{
			"acl_username":   acl.Username,
			"acl_permission": acl.Permission,
		}).Infof("Created ACL entry")
	}
	return nil
}

func (r *Manager) delete(toDelete []Acl) error {
	for _, acl := range toDelete {
		if len(acl.ID) == 0 {
			return fmt.Errorf("attemping to delete acl without ID: %v", acl)
		}
		err := r.AivenAdapter.Delete(acl.ID)
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
func NewACLs(existingAcls, wantedAcls []Acl) []Acl {
	candidates := make([]Acl, 0, len(wantedAcls))
	for _, wantedAcl := range wantedAcls {
		if !aclsContains(existingAcls, wantedAcl) {
			candidates = append(candidates, wantedAcl)
		}
	}
	return candidates
}

// DeleteACLs given a list of existing ACLs, return a new list of objects that don't exist in the cluster and should be deleted
func DeleteACLs(existingAcls, wantedAcls []Acl) []Acl {
	candidates := make([]Acl, 0, len(existingAcls))
	for _, existingAcl := range existingAcls {
		if !aclsContains(wantedAcls, existingAcl) {
			candidates = append(candidates, existingAcl)
		}
	}
	return candidates
}

func aclsContains(haystack []Acl, needle Acl) bool {
	for _, acl := range haystack {
		if needle.Equal(acl) {
			return true
		}
	}
	return false
}

// filter out ACLs not matching the topic pattern
func filterACLs(acls []Acl, topicPattern string) []Acl {
	result := make([]Acl, 0, len(acls))
	for _, acl := range acls {
		if acl.TopicPattern == topicPattern {
			result = append(result, acl)
		}
	}
	return result
}
