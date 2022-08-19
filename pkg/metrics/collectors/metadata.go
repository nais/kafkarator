package collectors

import (
	"context"
	"fmt"
	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/pkg/metrics"
	"github.com/nais/liberator/pkg/aiven/service"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"time"
)

type Metadata struct {
	Projects       []string
	Aiven          *aiven.Client
	Logger         log.FieldLogger
	ReportInterval time.Duration
	NameResolver   service.NameResolver
}

func formatError(err error) string {
	aivenError, ok := err.(aiven.Error)
	if !ok {
		return err.Error()
	} else {
		return fmt.Sprintf("%s: %s", aivenError.Message, aivenError.MoreInfo)
	}
}

func (m *Metadata) report(ctx context.Context) error {
	for _, project := range m.Projects {
		serviceName, err := m.NameResolver.ResolveKafkaServiceName(project)
		if err != nil {
			m.Logger.Errorf(formatError(err))
		}
		var svc *aiven.Service
		err = metrics.ObserveAivenLatency("Service_Get", project, func() error {
			var err error
			svc, err = m.Aiven.Services.Get(project, serviceName)
			return err
		})
		if err != nil {
			m.Logger.Error(formatError(err))
		} else {
			m.reportService(project, svc)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	return nil
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

func (m *Metadata) Run() {
	report := func() {
		ctx, cancel := context.WithTimeout(context.Background(), m.ReportInterval)
		now := time.Now()
		err := m.report(ctx)
		duration := time.Now().Sub(now)
		cancel()
		if err != nil {
			m.Logger.Errorf("Unable to report metadata metrics: %s", err)
		} else {
			m.Logger.Infof("Updated topic metadata in %s", duration)
		}
	}

	time.Sleep(time.Second * 5) // Wait 5 seconds before running first report, to allow Manager to start K8s Client
	report()
	ticker := time.NewTicker(m.ReportInterval)
	for range ticker.C {
		report()
	}
}
