package collectors

import (
	"context"
	"fmt"
	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/pkg/metrics"
	"github.com/nais/liberator/pkg/aiven/service"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Acls struct {
	client.Client
	projects     []string
	aiven        *aiven.Client
	logger       log.FieldLogger
	nameResolver service.NameResolver
}

func (a *Acls) Description() string {
	return "acl metrics"
}

func (a *Acls) Logger() log.FieldLogger {
	return a.logger
}

func (a *Acls) Report(ctx context.Context) error {
	clusterTopics := &kafka_nais_io_v1.TopicList{}
	err := a.List(ctx, clusterTopics)
	if err != nil {
		return fmt.Errorf("list kubernetes topics: %s", err)
	}

	for _, topic := range clusterTopics.Items {
		a.reportTopic(topic)
	}

	return nil
}

func (a *Acls) reportTopic(topic kafka_nais_io_v1.Topic) {
	type metric struct {
		topic string
		team  string
		app   string
		pool  string
	}

	uniq := make(map[metric]int)
	for _, acl := range topic.Spec.ACL {
		key := metric{
			topic: topic.Name,
			team:  acl.Team,
			app:   acl.Application,
			pool:  topic.Spec.Pool,
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
