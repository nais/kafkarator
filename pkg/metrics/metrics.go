package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	Topics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "topics",
		Namespace: "kafkarator",
		Help:      "number of topics",
	}, []string{"team"})
	TopicsProcessed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "kafkarator",
		Name:      "topics_processed",
		Help:      "number of topics synchronised with aiven",
	}, []string{"synchronisation_status"})
	AivenLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "kafkarator",
		Name:      "aiven_latency",
		Help:      "latency in aiven api operations",
		Buckets:   prometheus.LinearBuckets(0.05, 0.05, 100),
	}, []string{"operation"})
)
