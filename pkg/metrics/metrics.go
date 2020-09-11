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
	LabelPool           = "pool"
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

	ServiceUsers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "service_users",
		Namespace: Namespace,
		Help:      "number of service users",
	}, []string{LabelPool})

	AivenLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "aiven_latency",
		Namespace: Namespace,
		Help:      "latency in aiven api operations",
		Buckets:   prometheus.LinearBuckets(0.05, 0.05, 100),
	}, []string{LabelAivenOperation, LabelStatus, LabelPool})

	SecretQueueSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "secret_queue_size",
		Namespace: Namespace,
		Help:      "unwritten secrets for a specific group id",
	}, []string{LabelGroupID})
)

func ObserveAivenLatency(operation, pool string, fun func() error) error {
	timer := time.Now()
	err := fun()
	used := time.Now().Sub(timer)
	status := 200
	if err != nil {
		aivenErr := err.(aiven.Error)
		status = aivenErr.Status
	}
	AivenLatency.With(prometheus.Labels{
		LabelAivenOperation: operation,
		LabelPool:           pool,
		LabelStatus:         strconv.Itoa(status),
	}).Observe(used.Seconds())
	return err
}
