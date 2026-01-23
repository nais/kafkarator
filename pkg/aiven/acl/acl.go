package acl

import (
	"context"

	"github.com/nais/kafkarator/pkg/metrics"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	log "github.com/sirupsen/logrus"
)

type Interface interface {
	List(ctx context.Context, project, serviceName string) ([]*Acl, error)
	Create(ctx context.Context, project, service string, isStream bool, req CreateKafkaACLRequest) error
	Delete(ctx context.Context, project, service string, acl Acl) error
}

type Source interface {
	TopicName() string
	Pool() string
	ACLs() kafka_nais_io_v1.TopicACLs
	// TODO: find a better way to distinguish between Topic and Stream
	IsStream() bool
}

type Manager struct {
	AivenACLs        Interface
	Project          string
	Service          string
	Source           Source
	Logger           log.FieldLogger
	DryRun           bool
	DeleteLegacyACLs bool
}

// Synchronize Syncs the ACL spec in the Source resource with Aiven.
//
//	Missing ACL definitions are created, unnecessary definitions are deleted.
func (r *Manager) Synchronize(ctx context.Context) error {
	existingAcls, err := r.getExistingAcls(ctx)
	if err != nil {
		return err
	}

	wantedAcls, err := r.getWantedAcls(r.Source.TopicName(), r.Source.ACLs())
	if err != nil {
		return err
	}

	toAdd := NewACLs(existingAcls, wantedAcls)
	toDelete := DeleteACLs(existingAcls, wantedAcls)

	err = r.add(ctx, toAdd)
	if err != nil {
		return err
	}

	err = r.delete(ctx, toDelete)
	if err != nil {
		return err
	}

	return nil
}

func (r *Manager) getExistingAcls(ctx context.Context) ([]Acl, error) {
	var kafkaAcls []*Acl
	err := metrics.ObserveAivenLatency("ACL_List", r.Project, func() error {
		var err error
		kafkaAcls, err = r.AivenACLs.List(ctx, r.Project, r.Service)
		return err
	})
	if err != nil {
		return nil, err
	}

	acls := topicACLs(kafkaAcls, r.Source.TopicName())
	log.Info("Existing ACLs: ", acls)
	return acls, nil
}

func (r *Manager) getWantedAcls(topic string, topicAcls []kafka_nais_io_v1.TopicACL) ([]Acl, error) {
	wantedAcls := make([]Acl, 0, len(topicAcls))
	for _, aclSpec := range topicAcls {
		newNameAcl, err := FromTopicACL(topic, &aclSpec, func(topicAcl *kafka_nais_io_v1.TopicACL) (string, error) {
			return topicAcl.ServiceUserNameWithSuffix("*")
		})
		if err != nil {
			return nil, err
		}
		wantedAcls = append(wantedAcls, newNameAcl)
	}
	log.Info("Wanted ACLs: ", wantedAcls)
	return wantedAcls, nil
}

func (r *Manager) add(ctx context.Context, toAdd []Acl) error {
	for _, acl := range toAdd {
		req := CreateKafkaACLRequest{
			Permission: acl.Permission,
			Topic:      acl.Topic,
			Username:   acl.Username,
		}

		err := metrics.ObserveAivenLatency("ACL_Create", r.Project, func() error {
			var err error
			if r.DryRun {
				r.Logger.Infof("DRY RUN: Would create ACL entry: %v", req)
				return nil
			}

			if err = r.AivenACLs.Create(ctx, r.Project, r.Service, r.Source.IsStream(), req); err != nil {
				return err
			}
			return nil
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

func (r *Manager) delete(ctx context.Context, toDelete []Acl) error {
	for _, acl := range toDelete {

		deleteNative := len(acl.NativeIDs) > 0
		deleteLegacy := acl.ID != ""

		if !deleteNative && !deleteLegacy {
			r.Logger.WithFields(log.Fields{
				"id":             acl.ID,
				"native_ids":     acl.NativeIDs,
				"acl_username":   acl.Username,
				"acl_permission": acl.Permission,
				"acl_topic":      acl.Topic,
			}).Info("Skipping ACL delete (migration policy)")
			continue
		}

		err := metrics.ObserveAivenLatency("ACL_Delete", r.Project, func() error {
			if r.DryRun {
				r.Logger.Infof("DRY RUN: Would delete ACL entry: %v", acl)
				return nil
			}
			return r.AivenACLs.Delete(ctx, r.Project, r.Service, acl)
		})
		if err != nil {
			return err
		}

		r.Logger.WithFields(log.Fields{
			"id":             acl.ID,
			"native_ids":     acl.NativeIDs,
			"acl_username":   acl.Username,
			"acl_permission": acl.Permission,
			"acl_topic":      acl.Topic,
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

// filter out ACLs not matching the topic name
func topicACLs(acls []*Acl, topic string) []Acl {
	result := make([]Acl, 0, len(acls))
	for _, acl := range acls {
		if acl.Topic == topic {
			result = append(result, *acl)
		}
	}
	return result
}

type TopicAdapter struct {
	*kafka_nais_io_v1.Topic
}

func (t TopicAdapter) TopicName() string {
	return t.FullName()
}

func (t TopicAdapter) IsStream() bool {
	return false
}

func (t TopicAdapter) Pool() string {
	return t.Spec.Pool
}

func (t TopicAdapter) ACLs() kafka_nais_io_v1.TopicACLs {
	return t.Spec.ACL
}

type StreamAdapter struct {
	*kafka_nais_io_v1.Stream
	Delete bool
}

func (s StreamAdapter) TopicName() string {
	return s.TopicWildcard()
}

func (s StreamAdapter) IsStream() bool {
	return true
}

func (s StreamAdapter) Pool() string {
	return s.Spec.Pool
}

func (s StreamAdapter) ACLs() kafka_nais_io_v1.TopicACLs {
	if s.Delete {
		return kafka_nais_io_v1.TopicACLs{}
	}

	acls := kafka_nais_io_v1.TopicACLs{s.ACL()}
	for _, user := range s.Spec.AdditionalUsers {
		acls = append(acls, kafka_nais_io_v1.TopicACL{
			Access:      "admin",
			Application: user.Username,
			Team:        s.Namespace,
		})
	}

	return acls
}
