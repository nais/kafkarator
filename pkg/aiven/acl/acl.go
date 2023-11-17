package acl

import (
	"github.com/nais/kafkarator/pkg/aiven/acl/schemaregistry"
	"github.com/nais/kafkarator/pkg/aiven/acl/topic"
	"github.com/sirupsen/logrus"
)

type Manager struct {
	topicManager          topic.Manager
	schemaRegistryManager schemaregistry.Manager
}

func New(topicACLs topic.Interface,
	schemaRegistryACLs schemaregistry.Interface,
	project string,
	service string,
	topicSource topic.Source,
	schemaRegistrySource schemaregistry.Source,
	log logrus.FieldLogger) Manager {
	return Manager{
		topicManager: topic.Manager{
			AivenACLs: topicACLs,
			Project:   project,
			Service:   service,
			Source:    topicSource,
			Logger:    log,
		},
		schemaRegistryManager: schemaregistry.Manager{
			AivenSchemaACLs: schemaRegistryACLs,
			Project:         project,
			Service:         service,
			Source:          schemaRegistrySource,
			Logger:          log},
	}
}

func (m *Manager) Synchronize() error {
	if err := m.topicManager.Synchronize(); err != nil {
		return err
	}

	if err := m.schemaRegistryManager.Synchronize(); err != nil {
		return err
	}

	return nil
}
