package kafkarator_aiven

import (
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/kafkarator/pkg/aiven/topic"
	"github.com/nais/liberator/pkg/aiven/service"
)

type Interfaces struct {
	ACLs         acl.Interface
	Topics       topic.Interface
	NameResolver service.NameResolver
}
