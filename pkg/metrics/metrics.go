package metrics

import (
	"strconv"
	"time"

	"github.com/aiven/aiven-go-client"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	Namespace = "kafkarator"

	LabelTeam           = "team"
	LabelTopic          = "topic"
	LabelApp            = "app"
	LabelSyncState      = "synchronization_state"
	LabelAivenOperation = "operation"
	LabelStatus         = "status"
	LabelPool           = "pool"
)

var (
	Topics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "topics",
		Namespace: Namespace,
		Help:      "number of topics",
	}, []string{LabelTeam, LabelPool})

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
