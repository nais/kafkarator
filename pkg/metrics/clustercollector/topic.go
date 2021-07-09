package clustercollector

import (
	"context"
	"fmt"
	"time"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/pkg/aiven"
	"github.com/nais/kafkarator/pkg/aiven/topic"
	"github.com/nais/kafkarator/pkg/metrics"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Topic struct {
	client.Client
	Aiven          *aiven.Client
	Logger         *log.Logger
	ReportInterval time.Duration
}

func (t *Topic) existingTopics(ctx context.Context, topics []kafka_nais_io_v1.Topic) (map[string][]*aiven.KafkaListTopic, error) {
	var err error

	existing := make(map[string][]*aiven.KafkaListTopic)

	// make list of known pools
	for _, top := range topics {
		existing[top.Spec.Pool] = nil
	}

	// fetch existing topics
	for pool := range existing {
		topicManager := topic.Manager{
			AivenTopics: t.Aiven.KafkaTopics,
			Project:     pool,
			Service:     kafkarator_aiven.ServiceName(pool),
			Logger:      t.Logger.WithContext(ctx),
		}
		existing[pool], err = topicManager.List()
		if err != nil {
			return nil, err
		}
	}

	return existing, nil
}

func (t *Topic) Report(ctx context.Context) error {
	topics := &kafka_nais_io_v1.TopicList{}
	// listopts := &client.ListOptions{}
	err := t.List(ctx, topics)
	if err != nil {
		return fmt.Errorf("list kubernetes topics: %s", err)
	}

	existing, err := t.existingTopics(ctx, topics.Items)
	if err != nil {
		return fmt.Errorf("list aiven topics: %s", err)
	}

	type key struct {
		source string
		team   string
		pool   string
	}

	report := func(k key, count int) {
		metrics.Topics.With(prometheus.Labels{
			metrics.LabelSource: k.source,
			metrics.LabelTeam:   k.team,
			metrics.LabelPool:   k.pool,
		}).Set(float64(count))
	}

	reports := make(map[key]int)
	for _, top := range topics.Items {
		k := key{
			source: metrics.SourceCluster,
			team:   top.Namespace,
			pool:   top.Spec.Pool,
		}
		reports[k]++
	}

	for pool, tops := range existing {
		for _, top := range tops {
			k := key{
				source: metrics.SourceAiven,
				team:   topicTeam(top, topics.Items),
				pool:   pool,
			}
			reports[k]++
		}
	}

	for key, count := range reports {
		report(key, count)
	}

	return nil
}

func (t *Topic) Run() {
	report := func() {
		ctx, cancel := context.WithTimeout(context.Background(), t.ReportInterval)
		now := time.Now()
		err := t.Report(ctx)
		duration := time.Now().Sub(now)
		cancel()
		if err != nil {
			t.Logger.Errorf("Unable to report topic metrics: %s", err)
		} else {
			t.Logger.Infof("Updated topic metrics in %s", duration)
		}
	}

	report()
	ticker := time.NewTicker(t.ReportInterval)
	for range ticker.C {
		report()
	}
}

// look up an Aiven topic's team from the kubernetes topic specs
func topicTeam(aivenTopic *aiven.KafkaListTopic, clusterTopics []kafka_nais_io_v1.Topic) string {
	for _, top := range clusterTopics {
		if top.Name == aivenTopic.TopicName {
			return top.Namespace
		}
	}
	return ""
}
