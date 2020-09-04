package topic

import (
	"net/http"

	"github.com/nais/kafkarator/pkg/metrics"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/api/v1"
	log "github.com/sirupsen/logrus"
)

type Topic interface {
	Get(project, service, topic string) (*aiven.KafkaTopic, error)
	List(project, service string) ([]*aiven.KafkaListTopic, error)
	Create(project, service string, req aiven.CreateKafkaTopicRequest) error
	Update(project, service, topic string, req aiven.UpdateKafkaTopicRequest) error
}

type Manager struct {
	AivenTopics Topic
	Project     string
	Service     string
	Topic       kafka_nais_io_v1.Topic
	Logger      *log.Entry
}

func aivenError(err error) *aiven.Error {
	aivenErr, ok := err.(aiven.Error)
	if ok {
		return &aivenErr
	}
	return nil
}

// given a list of usernames, create Aiven users not found in that list
func (r *Manager) Synchronize() error {
	var topic *aiven.KafkaTopic
	err := metrics.ObserveAivenLatency("Topic_Get", r.Project, func() error {
		var err error
		topic, err = r.AivenTopics.Get(r.Project, r.Service, r.Topic.Name)
		return err
	})
	if err != nil {
		aivenErr := aivenError(err)
		if aivenErr != nil && aivenErr.Status == http.StatusNotFound {
			r.Logger.Infof("Topic does not exist")
			return r.create()
		}
		return err
	}

	// topic already exists
	if topicConfigChanged(topic, r.Topic.Spec.Config) {
		r.Logger.Infof("Topic already exists")
		return r.update()
	}

	return nil
}

func (r *Manager) List() ([]*aiven.KafkaListTopic, error) {
	var list []*aiven.KafkaListTopic
	err := metrics.ObserveAivenLatency("Topic_List", r.Project, func() error {
		var err error
		list, err = r.AivenTopics.List(r.Project, r.Service)
		return err
	})
	return list, err
}

func (r *Manager) create() error {
	r.Logger.Infof("Creating topic")

	cfg := r.Topic.Spec.Config
	if cfg == nil {
		cfg = &kafka_nais_io_v1.Config{}
	}

	req := aiven.CreateKafkaTopicRequest{
		CleanupPolicy:         cfg.CleanupPolicy,
		MinimumInSyncReplicas: cfg.MinimumInSyncReplicas,
		Partitions:            cfg.Partitions,
		Replication:           cfg.Replication,
		RetentionBytes:        cfg.RetentionBytes,
		RetentionHours:        cfg.RetentionHours,
		TopicName:             r.Topic.Name,
	}

	return metrics.ObserveAivenLatency("Topic_Create", r.Project, func() error {
		return r.AivenTopics.Create(r.Project, r.Service, req)
	})
}

func (r *Manager) update() error {
	r.Logger.Infof("Updating topic")

	cfg := r.Topic.Spec.Config
	// below code should never run - should not be nil due to topicConfigChanged()
	if cfg == nil {
		cfg = &kafka_nais_io_v1.Config{}
	}

	req := aiven.UpdateKafkaTopicRequest{
		MinimumInSyncReplicas: cfg.MinimumInSyncReplicas,
		Partitions:            cfg.Partitions,
		Replication:           cfg.Replication,
		RetentionBytes:        cfg.RetentionBytes,
		RetentionHours:        cfg.RetentionHours,
	}

	return metrics.ObserveAivenLatency("Topic_Update", r.Project, func() error {
		return r.AivenTopics.Update(r.Project, r.Service, r.Topic.Name, req)
	})
}

func topicConfigChanged(topic *aiven.KafkaTopic, config *kafka_nais_io_v1.Config) bool {
	if config == nil {
		return false
	}
	if config.RetentionHours != nil && topic.RetentionHours != *config.RetentionHours {
		return true
	}
	if config.RetentionBytes != nil && topic.RetentionBytes != *config.RetentionBytes {
		return true
	}
	if config.Replication != nil && topic.Replication != *config.Replication {
		return true
	}
	if config.Partitions != nil && len(topic.Partitions) != *config.Partitions {
		return true
	}
	if config.MinimumInSyncReplicas != nil && topic.MinimumInSyncReplicas != *config.MinimumInSyncReplicas {
		return true
	}
	return false
}
