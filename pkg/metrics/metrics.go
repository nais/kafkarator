package metrics

import (
	"strconv"
	"time"

	"github.com/aiven/aiven-go-client"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	Namespace = "kafkarator"

	LabelAivenOperation = "operation"
	LabelApp            = "app"
	LabelGroupID        = "group_id"
	LabelNamespace      = "namespace"
	LabelPool           = "pool"
	LabelResourceType   = "resource_type"
	LabelSource         = "source"
	LabelStatus         = "status"
	LabelSyncState      = "synchronization_state"
	LabelTeam           = "team"
	LabelTopic          = "topic"

	SourceCluster = "cluster"
	SourceAiven   = "aiven"
)

var (
	Topics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "topics",
		Namespace: Namespace,
		Help:      "number of topics",
	}, []string{LabelSource, LabelTeam, LabelPool})

	TopicsProcessed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "topics_processed",
		Namespace: Namespace,
		Help:      "number of topics synchronized with aiven",
	}, []string{LabelSyncState, LabelPool})

	Acls = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "acls",
		Namespace: Namespace,
		Help:      "number of acls",
	}, []string{LabelTopic, LabelTeam, LabelApp, LabelPool})

	AivenLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "aiven_latency",
		Namespace: Namespace,
		Help:      "latency in aiven api operations",
		Buckets:   []float64{.005, .010, .015, .020, .025, .030, .035, .040, .045, .050, .1, .2, .3, .4, .5, 1, 2, 3, 4, 5, 10, 15, 20},
	}, []string{LabelAivenOperation, LabelStatus, LabelPool})

	SecretQueueSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "secret_queue_size",
		Namespace: Namespace,
		Help:      "unwritten secrets for a specific group id",
	}, []string{LabelGroupID})

	KubernetesResourcesWritten = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "kubernetes_resources_written",
		Namespace: Namespace,
		Help:      "number of kubernetes resources written to the cluster",
	}, []string{LabelNamespace, LabelResourceType})
)

func ObserveAivenLatency(operation, pool string, fun func() error) error {
	timer := time.Now()
	err := fun()
	used := time.Now().Sub(timer)
	status := 200
	if err != nil {
		aivenErr, ok := err.(aiven.Error)
		if ok {
			status = aivenErr.Status
		} else {
			status = 0
		}
	}
	AivenLatency.With(prometheus.Labels{
		LabelAivenOperation: operation,
		LabelPool:           pool,
		LabelStatus:         strconv.Itoa(status),
	}).Observe(used.Seconds())
	return err
}

func Register(registry prometheus.Registerer) {
	registry.MustRegister(
		Acls,
		AivenLatency,
		KubernetesResourcesWritten,
		SecretQueueSize,
		Topics,
		TopicsProcessed,
	)
}
