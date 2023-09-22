package topic

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/intstr"
	"net/http"
	"time"

	"github.com/nais/kafkarator/pkg/metrics"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	log "github.com/sirupsen/logrus"
)

type Interface interface {
	Get(project, service, topic string) (*aiven.KafkaTopic, error)
	List(project, service string) ([]*aiven.KafkaListTopic, error)
	Create(project, service string, req aiven.CreateKafkaTopicRequest) error
	Update(project, service, topic string, req aiven.UpdateKafkaTopicRequest) error
	Delete(project, service, topic string) error
}

type Manager struct {
	AivenTopics Interface
	Project     string
	Service     string
	Topic       kafka_nais_io_v1.Topic
	Logger      log.FieldLogger
}

func aivenError(err error) *aiven.Error {
	aivenErr, ok := err.(aiven.Error)
	if ok {
		return &aivenErr
	}
	return nil
}

func (r *Manager) Synchronize() error {
	var topic *aiven.KafkaTopic
	err := metrics.ObserveAivenLatency("Topic_Get", r.Project, func() error {
		var err error
		topic, err = r.AivenTopics.Get(r.Project, r.Service, r.Topic.FullName())
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
	if intpBiggerThan(cfg.MinimumInSyncReplicas, cfg.Replication) {
		return fmt.Errorf("MinimumInSyncReplicas (%d) shouldn't be bigger than Replication (%d)",
			*cfg.MinimumInSyncReplicas, *cfg.Replication)
	}
	minCleanableDirtyRatio, err := percentToRatio(cfg.MinCleanableDirtyRatioPercent)
	if err != nil {
		return fmt.Errorf("failed to parse MinCleanableDirtyRatioPercent; must be a number 0-100 with optionally a percent sign: %w", err)
	}

	req := aiven.CreateKafkaTopicRequest{
		TopicName:   r.Topic.FullName(),
		Partitions:  cfg.Partitions,
		Replication: cfg.Replication,
		Config: aiven.KafkaTopicConfig{
			CleanupPolicy:          cleanupPolicy(cfg),
			MaxMessageBytes:        intpToInt64p(cfg.MaxMessageBytes),
			MinInsyncReplicas:      intpToInt64p(cfg.MinimumInSyncReplicas),
			RetentionBytes:         intpToInt64p(cfg.RetentionBytes),
			RetentionMs:            retentionMs(cfg),
			SegmentMs:              segmentMs(cfg),
			MinCleanableDirtyRatio: minCleanableDirtyRatio,
			MinCompactionLagMs:     intpToInt64p(cfg.MinCompactionLagMs),
			MaxCompactionLagMs:     intpToInt64p(cfg.MaxCompactionLagMs),
		},
		Tags: []aiven.KafkaTopicTag{
			{Key: "created-by", Value: "Kafkarator"},
			{Key: "touched-at", Value: time.Now().Format(time.RFC3339)},
		},
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
	if intpBiggerThan(cfg.MinimumInSyncReplicas, cfg.Replication) {
		return fmt.Errorf("MinimumInSyncReplicas (%d) shouldn't be bigger than Replication (%d)",
			*cfg.MinimumInSyncReplicas, *cfg.Replication)
	}
	minCleanableDirtyRatio, err := percentToRatio(cfg.MinCleanableDirtyRatioPercent)
	if err != nil {
		return fmt.Errorf("failed to parse MinCleanableDirtyRatioPercent; must be a number 0-100 with optionally a percent sign: %w", err)
	}

	req := aiven.UpdateKafkaTopicRequest{
		Partitions:  cfg.Partitions,
		Replication: cfg.Replication,
		Config: aiven.KafkaTopicConfig{
			CleanupPolicy:          cleanupPolicy(cfg),
			MaxMessageBytes:        intpToInt64p(cfg.MaxMessageBytes),
			MinInsyncReplicas:      intpToInt64p(cfg.MinimumInSyncReplicas),
			RetentionBytes:         intpToInt64p(cfg.RetentionBytes),
			RetentionMs:            retentionMs(cfg),
			SegmentMs:              segmentMs(cfg),
			MinCleanableDirtyRatio: minCleanableDirtyRatio,
			MinCompactionLagMs:     intpToInt64p(cfg.MinCompactionLagMs),
			MaxCompactionLagMs:     intpToInt64p(cfg.MaxCompactionLagMs),
		},
		Tags: []aiven.KafkaTopicTag{
			{Key: "created-by", Value: "Kafkarator"},
			{Key: "touched-at", Value: time.Now().Format(time.RFC3339)},
		},
	}

	return metrics.ObserveAivenLatency("Topic_Update", r.Project, func() error {
		return r.AivenTopics.Update(r.Project, r.Service, r.Topic.FullName(), req)
	})
}

func topicConfigChanged(topic *aiven.KafkaTopic, config *kafka_nais_io_v1.Config) bool {
	if config == nil {
		return false
	}

	if config.Replication != nil && topic.Replication != *config.Replication {
		return true
	}
	if config.Partitions != nil && len(topic.Partitions) != *config.Partitions {
		return true
	}

	if config.RetentionHours != nil && topic.Config.RetentionMs.Value != *retentionMs(config) {
		return true
	}
	if intPToValueChanged(config.RetentionBytes, topic.Config.RetentionBytes) {
		return true
	}
	if intPToValueChanged(config.MinimumInSyncReplicas, topic.Config.MinInsyncReplicas) {
		return true
	}
	if config.SegmentHours != nil && topic.Config.SegmentMs.Value != *segmentMs(config) {
		return true
	}
	if intPToValueChanged(config.MaxMessageBytes, topic.Config.MaxMessageBytes) {
		return true
	}

	if intPToValueChanged(config.MinCompactionLagMs, topic.Config.MinCompactionLagMs) {
		return true
	}
	if intPToValueChanged(config.MaxCompactionLagMs, topic.Config.MaxCompactionLagMs) {
		return true
	}
	if config.MinCleanableDirtyRatioPercent != nil {
		ratio, err := percentToRatio(config.MinCleanableDirtyRatioPercent)
		if err != nil || topic.Config.MinCleanableDirtyRatio.Value != *ratio {
			return true
		}

	}
	return false
}

func percentToRatio(percent *intstr.IntOrString) (*float64, error) {
	if percent == nil {
		return nil, nil
	}
	value, err := intstr.GetScaledValueFromIntOrPercent(percent, 100, false)
	if err != nil {
		return nil, err
	}
	if value <= 0 || 100 < value {
		return nil, fmt.Errorf("invalid percentage value (%d)", value)
	}
	ratio := float64(value) / 100.0
	return &ratio, nil
}

func intPToValueChanged(cfg *int, tcfg aiven.KafkaTopicConfigResponseInt) bool {
	return cfg != nil && tcfg.Value != int64(*cfg)
}

func retentionMs(cfg *kafka_nais_io_v1.Config) *int64 {
	var ret *int64
	if cfg.RetentionHours != nil {
		var ms int64
		if *cfg.RetentionHours < 0 {
			ms = -1
		} else {
			retentionDuration := time.Duration(*cfg.RetentionHours) * time.Hour
			ms = retentionDuration.Milliseconds()
		}
		ret = &ms
	}
	return ret
}

func segmentMs(cfg *kafka_nais_io_v1.Config) *int64 {
	if cfg.SegmentHours == nil {
		return nil
	}

	var segmentDuration time.Duration
	if *cfg.SegmentHours < 1 {
		segmentDuration = time.Duration(1) * time.Hour
	} else {
		segmentDuration = time.Duration(*cfg.SegmentHours) * time.Hour
	}

	ms := segmentDuration.Milliseconds()
	return &ms
}

func cleanupPolicy(cfg *kafka_nais_io_v1.Config) string {
	ret := ""
	if cfg.CleanupPolicy != nil {
		ret = *cfg.CleanupPolicy
	}
	return ret
}

func intpToInt64p(i *int) *int64 {
	if i == nil {
		return nil
	}
	r := int64(*i)
	return &r
}

func intpBiggerThan(first, second *int) bool {
	return first != nil && second != nil && *first > *second
}
