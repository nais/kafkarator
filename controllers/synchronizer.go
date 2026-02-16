package controllers

import (
	"context"

	"github.com/nais/kafkarator/pkg/aiven"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/kafkarator/pkg/aiven/topic"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	log "github.com/sirupsen/logrus"
)

type Synchronizer struct {
	ACLs   acl.Manager
	Topics topic.Manager
	Logger *log.Entry
}

func NewSynchronizer(ctx context.Context, a kafkarator_aiven.Interfaces, t kafka_nais_io_v1.Topic, logger *log.Entry, dryRun bool) (*Synchronizer, error) {
	projectName := t.Spec.Pool
	serviceName, err := a.NameResolver.ResolveKafkaServiceName(ctx, projectName)
	if err != nil {
		return nil, err
	}

	return &Synchronizer{
		Logger: logger,
		Topics: topic.Manager{
			AivenTopics: a.Topics,
			Project:     projectName,
			Service:     serviceName,
			Topic:       t,
			Logger:      logger,
			DryRun:      dryRun,
		},
		ACLs: acl.Manager{
			AivenACLs: a.ACLs,
			Project:   projectName,
			Service:   serviceName,
			Source:    acl.TopicAdapter{Topic: &t},
			Logger:    logger,
			DryRun:    dryRun,
		},
	}, nil
}

func (c *Synchronizer) Synchronize(ctx context.Context) error {
	c.Logger.Infof("Synchronizing access control lists")
	err := c.ACLs.Synchronize(ctx)
	if err != nil {
		return err
	}

	c.Logger.Infof("Synchronizing topic")
	err = c.Topics.Synchronize(ctx)
	if err != nil {
		return err
	}

	return nil
}
