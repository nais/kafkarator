package collectors

import (
	"context"
	"fmt"
	"github.com/aiven/aiven-go-client/v2"
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

type metric struct {
	topic  string
	pool   string
	source string
}

func (a *Acls) Description() string {
	return "acl metrics"
}

func (a *Acls) Logger() log.FieldLogger {
	return a.logger
}

func (a *Acls) Report(ctx context.Context) error {
	err := a.reportFromClusterTopics(ctx)
	if err != nil {
		return err
	}

	return a.reportFromAivenProjects(ctx)
}

func (a *Acls) reportFromAivenProjects(ctx context.Context) error {
	for _, project := range a.projects {
		svcName, err := a.nameResolver.ResolveKafkaServiceName(ctx, project)
		if err != nil {
			return fmt.Errorf("resolve kafka service name in project %s: %s", project, err)
		}

		acls, err := a.aiven.KafkaACLs.List(ctx, project, svcName)
		if err != nil {
			return fmt.Errorf("list acls in in project %s: %s", project, err)
		}

		topics := make(map[string][]*aiven.KafkaACL)
		for _, kafkaACL := range acls {
			topics[kafkaACL.Topic] = append(topics[kafkaACL.Topic], kafkaACL)
		}

		for topicName, kafkaACLS := range topics {
			metrics.Acls.With(prometheus.Labels{
				metrics.LabelTopic:  topicName,
				metrics.LabelPool:   project,
				metrics.LabelSource: metrics.SourceAiven,
			}).Set(float64(len(kafkaACLS)))
		}
	}
	return nil
}

func (a *Acls) reportFromClusterTopics(ctx context.Context) error {
	clusterTopics := &kafka_nais_io_v1.TopicList{}
	err := a.List(ctx, clusterTopics)
	if err != nil {
		return fmt.Errorf("list kubernetes topics: %s", err)
	}

	for _, topic := range clusterTopics.Items {
		metrics.Acls.With(prometheus.Labels{
			metrics.LabelTopic:  topic.Name,
			metrics.LabelPool:   topic.Spec.Pool,
			metrics.LabelSource: metrics.SourceCluster,
		}).Set(float64(len(topic.Spec.ACL)))
	}

	return nil
}
