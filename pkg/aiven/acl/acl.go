package acl

import (
	"fmt"
	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/pkg/metrics"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type Interface interface {
	List(project, serviceName string) ([]*aiven.KafkaACL, error)
	Create(project, service string, req aiven.CreateKafkaACLRequest) (*aiven.KafkaACL, error)
	Delete(project, service, aclID string) error
}

type Source interface {
	TopicName() string
	Pool() string
	ACLs() kafka_nais_io_v1.TopicACLs
}

type Manager struct {
	AivenACLs Interface
	Project   string
	Service   string
	Source    Source
	Logger    log.FieldLogger
}

// Synchronize Syncs the ACL spec in the Source resource with Aiven.
// 			   Missing ACL definitions are created, unnecessary definitions are deleted.
func (r *Manager) Synchronize() error {
	existingAcls, err := r.getExistingAcls()
	if err != nil {
		return err
	}

	wantedAcls, err := r.getWantedAcls(r.Source.TopicName(), r.Source.ACLs())
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

	r.reportMetrics()

	return nil
}

func (r *Manager) getExistingAcls() ([]Acl, error) {
	var kafkaAcls []*aiven.KafkaACL
	err := metrics.ObserveAivenLatency("ACL_List", r.Project, func() error {
		var err error
		kafkaAcls, err = r.AivenACLs.List(r.Project, r.Service)
		return err
	})
	if err != nil {
		return nil, err
	}

	acls := topicACLs(kafkaAcls, r.Source.TopicName())
	return acls, nil
}

func (r *Manager) getWantedAcls(topic string, topicAcls []kafka_nais_io_v1.TopicACL) ([]Acl, error) {
	wantedAcls := make([]Acl, 0, len(topicAcls))
	for _, aclSpec := range topicAcls {
		oldNameAcl, err := FromTopicACL(topic, &aclSpec, func(topicAcl *kafka_nais_io_v1.TopicACL) (string, error) {
			return topicAcl.ACLname(), nil
		})
		if err != nil {
			return nil, err
		}
		wantedAcls = append(wantedAcls, oldNameAcl)

		newNameAcl, err := FromTopicACL(topic, &aclSpec, func(topicAcl *kafka_nais_io_v1.TopicACL) (string, error) {
			return topicAcl.ServiceUserNameWithSuffix("*")
		})
		if err != nil {
			return nil, err
		}
		wantedAcls = append(wantedAcls, newNameAcl)
	}
	return wantedAcls, nil
}

func (r *Manager) reportMetrics() {
	type metric struct {
		topic string
		team  string
		app   string
		pool  string
	}

	uniq := make(map[metric]int)
	for _, acl := range r.Source.ACLs() {
		key := metric{
			topic: r.Source.TopicName(),
			team:  acl.Team,
			app:   acl.Application,
			pool:  r.Source.Pool(),
		}
		uniq[key]++
	}

	for key, count := range uniq {
		metrics.Acls.With(prometheus.Labels{
			metrics.LabelTopic: key.topic,
			metrics.LabelTeam:  key.team,
			metrics.LabelApp:   key.app,
			metrics.LabelPool:  key.pool,
		}).Set(float64(count))
	}
}

func (r *Manager) add(toAdd []Acl) error {
	for _, acl := range toAdd {
		req := aiven.CreateKafkaACLRequest{
			Permission: acl.Permission,
			Topic:      acl.Topic,
			Username:   acl.Username,
		}

		err := metrics.ObserveAivenLatency("ACL_Create", r.Project, func() error {
			var err error
			_, err = r.AivenACLs.Create(r.Project, r.Service, req)
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
		err := metrics.ObserveAivenLatency("ACL_Delete", r.Project, func() error {
			return r.AivenACLs.Delete(r.Project, r.Service, acl.ID)
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

// filter out ACLs not matching the topic name
func topicACLs(acls []*aiven.KafkaACL, topic string) []Acl {
	result := make([]Acl, 0, len(acls))
	for _, acl := range acls {
		if acl.Topic == topic {
			result = append(result, FromKafkaACL(acl))
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

func (s StreamAdapter) Pool() string {
	return s.Spec.Pool
}

func (s StreamAdapter) ACLs() kafka_nais_io_v1.TopicACLs {
	if s.Delete {
		return kafka_nais_io_v1.TopicACLs{}
	} else {
		return kafka_nais_io_v1.TopicACLs{s.ACL()}
	}
}
