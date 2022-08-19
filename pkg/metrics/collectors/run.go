package collectors

import (
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
		Client:         opts.Client,
		Aiven:          opts.AivenClient,
		Logger:         opts.Logger.WithField("metric-collector", "topic"),
		ReportInterval: opts.ReportInterval,
		NameResolver:   opts.NameResolver,
	}
	go topicCollector.Run()

	metadataCollector := &Metadata{
		Projects:       opts.Projects,
		Aiven:          opts.AivenClient,
		Logger:         opts.Logger.WithField("metric-collector", "metadata"),
		ReportInterval: opts.ReportInterval,
		NameResolver:   opts.NameResolver,
	}
	go metadataCollector.Run()
}
