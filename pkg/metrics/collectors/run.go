package collectors

import (
	"github.com/aiven/aiven-go-client"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func Start(client client.Client, aivenClient *aiven.Client, logger *logrus.Logger, reportInterval time.Duration, projects []string) {
	topicCollector := &Topic{
		Client:         client,
		Aiven:          aivenClient,
		Logger:         logger.WithField("metric-collector", "topic"),
		ReportInterval: reportInterval,
	}
	go topicCollector.Run()

	metadataCollector := &Metadata{
		Projects:       projects,
		Aiven:          aivenClient,
		Logger:         logger.WithField("metric-collector", "metadata"),
		ReportInterval: reportInterval,
	}
	go metadataCollector.Run()
}
