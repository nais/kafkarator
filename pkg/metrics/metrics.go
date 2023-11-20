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

	// Metadata labels
	LabelVersion = "version"
	LabelPlan    = "plan"

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

	StreamsProcessed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "streams_processed",
		Namespace: Namespace,
		Help:      "number of streams synchronized with aiven",
	}, []string{LabelSyncState, LabelPool})

	Acls = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "acls",
		Namespace: Namespace,
		Help:      "number of acls",
	}, []string{LabelTopic, LabelPool, LabelSource})

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

	PoolNodes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "kafka_pool_nodes_count",
		Namespace: Namespace,
		Help:      "number of nodes in the kafka pool",
	}, []string{LabelPool})

	PoolInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "kafka_pool_info",
		Namespace: Namespace,
		Help:      "metadata about kafka pool",
	}, []string{LabelPool, LabelVersion, LabelPlan})
)

func GObserveAivenLatency[T any](operation, pool string, fun func() (T, error)) (T, error) {
	timer := time.Now()
	value, err := fun()
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
	return value, err
}
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
		SecretQueueSize,
		Topics,
		TopicsProcessed,
		StreamsProcessed,
		PoolNodes,
		PoolInfo,
	)
}
