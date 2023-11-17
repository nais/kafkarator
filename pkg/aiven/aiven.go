package kafkarator_aiven

import (
	acl_schemaregistry "github.com/nais/kafkarator/pkg/aiven/acl/schemaregistry"
	acl_topic "github.com/nais/kafkarator/pkg/aiven/acl/topic"
	"github.com/nais/kafkarator/pkg/aiven/topic"
	"github.com/nais/liberator/pkg/aiven/service"
)

func ServiceName(project string) string {
	return project + "-kafka"
}

type Interfaces struct {
	TopicACLs          acl_topic.Interface
	SchemaRegistryACLs acl_schemaregistry.Interface
	Topics             topic.Interface
	NameResolver       service.NameResolver
}
