package acl

import (
	"github.com/nais/kafkarator/pkg/aiven/acl/manager"
	"github.com/sirupsen/logrus"
)

type Manager struct {
	topicManager             manager.Manager
	schemaRegistryManager    manager.Manager
	schemaRegistryACLEnabled bool
}

func New(kafkaAcls manager.KafkaAclInterface,
	schemaRegistryACLs manager.SchemaRegistryAclInterface,
	schemaRegistryACLEnabled bool,
	project string,
	service string,
	source manager.Source,
	log logrus.FieldLogger) Manager {

	m := Manager{schemaRegistryACLEnabled: schemaRegistryACLEnabled}
	m.topicManager = manager.Manager{
		AivenAdapter: manager.AivenKafkaAclAdapter{
			KafkaAclInterface: kafkaAcls,
			Project:           project,
			Service:           service,
		},
		Source: source,
		Logger: log,
	}

	if schemaRegistryACLEnabled {
		m.schemaRegistryManager = manager.Manager{
			AivenAdapter: manager.AivenSchemaRegistryACLAdapter{
				SchemaRegistryAclInterface: schemaRegistryACLs,
				Project:                    project,
				Service:                    service,
			},
			Source: source,
			Logger: log,
		}
	}
	return m
}

func (m *Manager) Synchronize() error {
	if err := m.topicManager.Synchronize(); err != nil {
		return err
	}

	if m.schemaRegistryACLEnabled {
		if err := m.schemaRegistryManager.Synchronize(); err != nil {
			return err
		}
	}

	return nil
}
