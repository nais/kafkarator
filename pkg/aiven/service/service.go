package service

import (
	"github.com/aiven/aiven-go-client"
	log "github.com/sirupsen/logrus"
)

type Service interface {
	Get(project, service string) (*aiven.Service, error)
}

type CA interface {
	Get(project string) (string, error)
}

type Manager struct {
	AivenService Service
	AivenCA      CA
	Project      string
	Service      string
	Logger       *log.Entry
}

func (r *Manager) Get() (*aiven.Service, error) {
	return r.AivenService.Get(r.Project, r.Service)
}

func (r *Manager) GetCA() (string, error) {
	return r.AivenCA.Get(r.Project)
}

func GetKafkaBrokerAddress(service aiven.Service) string {
	return service.URI
}

func GetSchemaRegistryAddress(service aiven.Service) string {
	return service.ConnectionInfo.SchemaRegistryURI
}
