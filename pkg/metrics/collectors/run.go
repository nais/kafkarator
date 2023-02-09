package collectors

import (
	"context"
	"github.com/aiven/aiven-go-client"
	"github.com/nais/liberator/pkg/aiven/service"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type Opts struct {
	Client         client.Client
	AivenClient    *aiven.Client
	ReportInterval time.Duration
	Projects       []string
	NameResolver   service.NameResolver
	Logger         logrus.FieldLogger
}

func Start(opts *Opts) {
	topicCollector := &Topic{
		Client:       opts.Client,
		projects:     opts.Projects,
		aiven:        opts.AivenClient,
		logger:       opts.Logger.WithField("metric-collector", "topic"),
		nameResolver: opts.NameResolver,
	}
	go run(topicCollector, opts.ReportInterval)

	metadataCollector := &Metadata{
		projects:     opts.Projects,
		aiven:        opts.AivenClient,
		logger:       opts.Logger.WithField("metric-collector", "metadata"),
		nameResolver: opts.NameResolver,
	}
	go run(metadataCollector, opts.ReportInterval)
}

func run(collector Collector, reportInterval time.Duration) {
	report := func() {
		ctx, cancel := context.WithTimeout(context.Background(), reportInterval)
		now := time.Now()
		err := collector.Report(ctx)
		duration := time.Now().Sub(now)
		cancel()
		if err != nil {
			collector.Logger().Errorf("Unable to report %s: %s", collector.Description(), err)
		} else {
			collector.Logger().Infof("Updated %s in %s", collector.Description(), duration)
		}
	}

	time.Sleep(time.Second * 5) // Wait 5 seconds before running first report, to allow Manager to start K8s Client
	report()
	ticker := time.NewTicker(reportInterval)
	for range ticker.C {
		report()
	}
}
