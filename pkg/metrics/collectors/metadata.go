package collectors

import (
	"context"
	"fmt"
	"github.com/aiven/aiven-go-client/v2"
	"github.com/nais/kafkarator/pkg/metrics"
	"github.com/nais/liberator/pkg/aiven/service"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type Metadata struct {
	projects     []string
	aiven        *aiven.Client
	logger       log.FieldLogger
	nameResolver service.NameResolver
}

func (m *Metadata) Description() string {
	return "metadata metrics"
}

func (m *Metadata) Logger() log.FieldLogger {
	return m.logger
}

func (m *Metadata) Report(ctx context.Context) error {
	for _, project := range m.projects {
		serviceName, err := m.nameResolver.ResolveKafkaServiceName(ctx, project)
		if err != nil {
			m.logger.Errorf(formatError(err))
		}
		var svc *aiven.Service
		err = metrics.ObserveAivenLatency("Service_Get", project, func() error {
			var err error
			svc, err = m.aiven.Services.Get(ctx, project, serviceName)
			return err
		})
		if err != nil {
			m.logger.Error(formatError(err))
		} else {
			m.reportService(project, svc)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	return nil
}

func formatError(err error) string {
	aivenError, ok := err.(aiven.Error)
	if !ok {
		return err.Error()
	} else {
		return fmt.Sprintf("%s: %s", aivenError.Message, aivenError.MoreInfo)
	}
}

func (m *Metadata) reportService(project string, svc *aiven.Service) {
	version, ok := svc.UserConfig["kafka_version"].(string)
	if !ok {
		version = "unknown"
	}
	metrics.PoolInfo.With(prometheus.Labels{
		metrics.LabelPool:    project,
		metrics.LabelVersion: version,
		metrics.LabelPlan:    svc.Plan,
	}).Set(1.0)
	metrics.PoolNodes.With(prometheus.Labels{
		metrics.LabelPool: project,
	}).Set(float64(svc.NodeCount))
}
