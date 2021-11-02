package controllers

import (
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

type SyncResult struct {
	brokers  string
	registry string
	ca       string
	topic    kafka_nais_io_v1.Topic
}

func NewSynchronizer(a kafkarator_aiven.Interfaces, t kafka_nais_io_v1.Topic, logger *log.Entry) *Synchronizer {
	return &Synchronizer{
		Logger: logger,
		Topics: topic.Manager{
			AivenTopics: a.Topics,
			Project:     t.Spec.Pool,
			Service:     kafkarator_aiven.ServiceName(t.Spec.Pool),
			Topic:       t,
			Logger:      logger,
		},
		ACLs: acl.Manager{
			AivenACLs: a.ACLs,
			Project:   t.Spec.Pool,
			Service:   kafkarator_aiven.ServiceName(t.Spec.Pool),
			Source:    acl.TopicAdapter{Topic: &t},
			Logger:    logger,
		},
	}
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
