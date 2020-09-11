package secretsync

import (
	"context"
	"time"

	"github.com/nais/kafkarator/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Synchronizer struct {
	client.Client
	Timeout time.Duration
}

func (s *Synchronizer) Write(secret *v1.Secret, logger *log.Entry) error {
	key := client.ObjectKey{
		Namespace: secret.Namespace,
		Name:      secret.Name,
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
	defer cancel()

	old := &v1.Secret{}
	err := s.Get(ctx, key, old)

	if err != nil {
		if errors.IsNotFound(err) {
			logger.Infof("Creating secret")
			err = s.Create(ctx, secret)
		}
	} else {
		logger.Infof("Updating secret")
		secret.ResourceVersion = old.ResourceVersion
		err = s.Update(ctx, secret)
	}

	if err == nil {
		metrics.KubernetesResourcesWritten.With(prometheus.Labels{
			metrics.LabelResourceType: "secret",
			metrics.LabelNamespace:    key.Namespace,
		})
	}

	return err
}
