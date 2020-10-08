package service

import (
	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/pkg/metrics"
	log "github.com/sirupsen/logrus"
)

type Interface interface {
	Get(project, service string) (*aiven.Service, error)
}

type CA interface {
	Get(project string) (string, error)
}

type Manager struct {
	AivenService Interface
	AivenCA      CA
	Project      string
	Service      string
	Logger       *log.Entry
}

func (r *Manager) Get() (*aiven.Service, error) {
	var service *aiven.Service
	err := metrics.ObserveAivenLatency("Service_Get", r.Project, func() error {
		var err error
		service, err = r.AivenService.Get(r.Project, r.Service)
		return err
	})
	return service, err
}

func (r *Manager) GetCA() (string, error) {
	var ca string
	err := metrics.ObserveAivenLatency("CA_Get", r.Project, func() error {
		var err error
		ca, err = r.AivenCA.Get(r.Project)
		return err
	})
	return ca, err
}

func GetKafkaBrokerAddress(service aiven.Service) string {
	return service.URI
}

func GetSchemaRegistryAddress(service aiven.Service) string {
	return service.ConnectionInfo.SchemaRegistryURI
}
