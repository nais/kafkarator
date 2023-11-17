package acl

import (
	"github.com/nais/kafkarator/pkg/aiven/acl/manager"
	"github.com/sirupsen/logrus"
)

type Manager struct {
	topicManager          manager.Manager
	schemaRegistryManager manager.Manager
}

func New(kafkaAcls manager.KafkaAclInterface,
	schemaRegistryACLs manager.SchemaRegistryAclInterface,
	project string,
	service string,
	source manager.Source,
	log logrus.FieldLogger) Manager {
	return Manager{
		topicManager: manager.Manager{
			AivenAdapter: manager.AivenKafkaAclAdapter{
				KafkaAclInterface: kafkaAcls,
				Project:           project,
				Service:           service,
			},
			Source: source,
			Logger: log,
		},
		schemaRegistryManager: manager.Manager{
			AivenAdapter: manager.AivenSchemaRegistryACLAdapter{
				SchemaRegistryAclInterface: schemaRegistryACLs,
				Project:                    project,
				Service:                    service,
			},
			Source: source,
			Logger: log},
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
