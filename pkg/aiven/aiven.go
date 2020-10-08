package kafkarator_aiven

import (
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/kafkarator/pkg/aiven/service"
	"github.com/nais/kafkarator/pkg/aiven/serviceuser"
	"github.com/nais/kafkarator/pkg/aiven/topic"
)

func ServiceName(project string) string {
	return project + "-kafka"
}

type Interfaces struct {
	ACLs         acl.Interface
	CA           service.CA
	ServiceUsers serviceuser.Interface
	Service      service.Interface
	Topics       topic.Interface
}
