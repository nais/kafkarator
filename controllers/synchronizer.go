package controllers

import (
	kafkarator_aiven "github.com/nais/kafkarator/pkg/aiven"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/kafkarator/pkg/aiven/acl/manager"
	"github.com/nais/kafkarator/pkg/aiven/topic"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	log "github.com/sirupsen/logrus"
)

type Synchronizer struct {
	ACLs   acl.Manager
	Topics topic.Manager
	Logger *log.Entry
}

type SyncResult struct {
	brokers  string
	registry string
	ca       string
	topic    kafka_nais_io_v1.Topic
}

func NewSynchronizer(a kafkarator_aiven.Interfaces, t kafka_nais_io_v1.Topic, logger *log.Entry) (*Synchronizer, error) {
	projectName := t.Spec.Pool
	serviceName, err := a.NameResolver.ResolveKafkaServiceName(projectName)
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
		},
		ACLs: acl.New(
			a.KafkaAcls,
			a.SchemaRegistryAcls,
			projectName,
			serviceName,
			manager.TopicAdapter{Topic: &t},
			logger,
		),
	}, nil
}

func (c *Synchronizer) Synchronize() error {
	c.Logger.Infof("Synchronizing access control lists")
	err := c.ACLs.Synchronize()
	if err != nil {
		return err
	}

	c.Logger.Infof("Synchronizing topic")
	err = c.Topics.Synchronize()
	if err != nil {
		return err
	}

	return nil
}
