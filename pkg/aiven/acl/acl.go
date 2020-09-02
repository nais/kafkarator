package acl

import (
	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/api/v1"
	log "github.com/sirupsen/logrus"
)

type Manager struct {
	Aiven   *aiven.Client
	Project string
	Service string
	Topic   kafka_nais_io_v1.Topic
	Logger  *log.Entry
}

// Sync the ACL spec in the Topic resource with Aiven.
// Missing ACL definitions are created, unneccessary definitions are deleted.
func (r *Manager) Synchronize() error {
	acls, err := r.Aiven.KafkaACLs.List(r.Project, r.Service)
	if err != nil {
		return err
	}

	acls = topicACLs(acls, r.Topic.Name)
	toAdd := NewACLs(acls, r.Topic.Spec.ACL)
	toDelete := DeleteACLs(acls, r.Topic.Spec.ACL)

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

func (r *Manager) add(toAdd []kafka_nais_io_v1.TopicACL) error {
	for _, topicAcl := range toAdd {
		req := aiven.CreateKafkaACLRequest{
			Permission: topicAcl.Access,
			Topic:      r.Topic.Name,
			Username:   topicAcl.Username(),
		}

		_, err := r.Aiven.KafkaACLs.Create(r.Project, r.Service, req)
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

func (r *Manager) delete(toDelete []*aiven.KafkaACL) error {
	for _, kafkaAcl := range toDelete {
		err := r.Aiven.KafkaACLs.Delete(r.Project, r.Service, kafkaAcl.ID)

		if err != nil {
			return err
		}

		r.Logger.WithFields(log.Fields{
			"acl_id":         kafkaAcl.ID,
			"acl_username":   kafkaAcl.Username,
			"acl_permission": kafkaAcl.Permission,
		}).Infof("Deleted ACL entry")
	}
	return nil
}

// given a list of ACL specs, return a new list of ACL objects that does not already exist
func NewACLs(acls []*aiven.KafkaACL, aclSpecs []kafka_nais_io_v1.TopicACL) []kafka_nais_io_v1.TopicACL {
	candidates := make([]kafka_nais_io_v1.TopicACL, 0, len(aclSpecs))
	for _, aclSpec := range aclSpecs {
		if !aclsContainsSpec(acls, aclSpec) {
			candidates = append(candidates, aclSpec)
		}
	}
	return candidates
}

// given a list of existing ACLs, return a new list of objects that don't exist in the cluster and should be deleted
func DeleteACLs(acls []*aiven.KafkaACL, aclSpecs []kafka_nais_io_v1.TopicACL) []*aiven.KafkaACL {
	candidates := make([]*aiven.KafkaACL, 0, len(acls))
	for _, acl := range acls {
		if !specsContainsACL(aclSpecs, acl) {
			candidates = append(candidates, acl)
		}
	}
	return candidates
}

// filter out ACLs not matching the topic name
func topicACLs(acls []*aiven.KafkaACL, topic string) []*aiven.KafkaACL {
	result := make([]*aiven.KafkaACL, 0, len(acls))
	for _, acl := range acls {
		if acl.Topic == topic {
			result = append(result, acl)
		}
	}
	return result
}

// returns true if the list of existing ACLs contains an ACL spec from the cluster
func aclsContainsSpec(acls []*aiven.KafkaACL, aclSpec kafka_nais_io_v1.TopicACL) bool {
	for _, acl := range acls {
		if aclSpec.Username() == acl.Username && aclSpec.Access == acl.Permission {
			return true
		}
	}
	return false
}

// returns true if the list of cluster ACL specs contains an existing ACL
func specsContainsACL(aclSpecs []kafka_nais_io_v1.TopicACL, acl *aiven.KafkaACL) bool {
	for _, aclSpec := range aclSpecs {
		if aclSpec.Username() == acl.Username && aclSpec.Access == acl.Permission {
			return true
		}
	}
	return false
}
