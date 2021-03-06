package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/pkg/aiven"
	"github.com/nais/kafkarator/pkg/metrics"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	LogFieldSynchronizationState = "synchronization_state"
)

type ReconcileResult struct {
	DeleteFinalized bool
	Skipped         bool
	Requeue         bool
	Status          kafka_nais_io_v1.TopicStatus
	Error           error
}

type TopicReconciler struct {
	client.Client
	Aiven           kafkarator_aiven.Interfaces
	Logger          *log.Logger
	Projects        []string
	RequeueInterval time.Duration
}

func (r *TopicReconciler) projectWhitelisted(project string) bool {
	for _, p := range r.Projects {
		if p == project {
			return true
		}
	}
	return false
}

// Process changes in Aiven and return a topic processing status
func (r *TopicReconciler) Process(topic kafka_nais_io_v1.Topic, logger *log.Entry) ReconcileResult {
	var err error
	var hash string
	var status kafka_nais_io_v1.TopicStatus

	if topic.Status != nil {
		status = *topic.Status
	}

	status.FullyQualifiedName = topic.FullName()

	fail := func(err error, state string, retry bool) ReconcileResult {
		aivenError, ok := err.(aiven.Error)
		if !ok {
			status.Message = err.Error()
			status.Errors = []string{
				err.Error(),
			}
		} else {
			status.Message = aivenError.Message
			status.Errors = []string{
				fmt.Sprintf("%s: %s", aivenError.Message, aivenError.MoreInfo),
			}
		}
		status.SynchronizationState = state

		return ReconcileResult{
			Requeue: retry,
			Status:  status,
			Error:   fmt.Errorf("%s: %s", state, err),
		}
	}

	// Process or delete?
	if topic.ObjectMeta.DeletionTimestamp != nil {
		if topic.RemoveDataWhenDeleted() {
			logger.Infof("Permanently deleting Aiven topic and its data")
			err = r.Aiven.Topics.Delete(topic.Spec.Pool, kafkarator_aiven.ServiceName(topic.Spec.Pool), topic.FullName())
			if err != nil {
				return fail(fmt.Errorf("failed to delete topic on Aiven: %s", err), kafka_nais_io_v1.EventFailedSynchronization, true)
			}
			status.Message = "Topic and data permanently deleted"
		}
		status.SynchronizationTime = time.Now().Format(time.RFC3339)
		status.Errors = nil
		return ReconcileResult{
			DeleteFinalized: true,
			Status:          status,
		}
	}

	hash, err = topic.Hash()
	if err != nil {
		return fail(fmt.Errorf("unable to calculate synchronization hash"), kafka_nais_io_v1.EventFailedPrepare, false)
	}

	if !topic.NeedsSynchronization(hash) {
		logger.Infof("Synchronization already complete")
		return ReconcileResult{
			Skipped: true,
		}
	}

	if !r.projectWhitelisted(topic.Spec.Pool) {
		return fail(fmt.Errorf("pool '%s' cannot be used in this cluster", topic.Spec.Pool), kafka_nais_io_v1.EventFailedPrepare, false)
	}

	synchronizer := NewSynchronizer(r.Aiven, topic, logger)
	err = synchronizer.Synchronize()
	if err != nil {
		return fail(err, kafka_nais_io_v1.EventFailedSynchronization, true)
	}

	status.SynchronizationTime = time.Now().Format(time.RFC3339)
	status.SynchronizationState = kafka_nais_io_v1.EventRolloutComplete
	status.SynchronizationHash = hash
	status.Message = "Topic configuration synchronized to Kafka pool"
	status.Errors = nil

	return ReconcileResult{
		Status: status,
	}
}

// +kubebuilder:rbac:groups=kafka.nais.io,resources=topics,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.nais.io,resources=topics/status,verbs=get;update;patch

func (r *TopicReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	var topic kafka_nais_io_v1.Topic

	logger := log.NewEntry(r.Logger)

	logger = logger.WithFields(log.Fields{
		"topic":     req.Name,
		"namespace": req.Namespace,
	})

	logger.Infof("Processing request")
	defer func() {
		logger.Infof("Finished processing request")
	}()

	ctx := context.Background()

	fail := func(err error, requeue bool) (ctrl.Result, error) {
		logger.Error(err)
		cr := &ctrl.Result{}
		if requeue {
			cr.RequeueAfter = r.RequeueInterval
		}
		return *cr, nil
	}

	err := r.Get(ctx, req.NamespacedName, &topic)
	switch {
	case errors.IsNotFound(err):
		return fail(fmt.Errorf("resource deleted from cluster; noop"), false)
	case err != nil:
		return fail(fmt.Errorf("unable to retrieve resource from cluster: %s", err), true)
	}

	logger = logger.WithFields(log.Fields{
		"team":          topic.Labels["team"],
		"aiven_project": topic.Spec.Pool,
		"aiven_topic":   topic.FullName(),
	})

	if topic.Status != nil {
		logger.Infof("Last synchronization time: %s", topic.Status.SynchronizationTime)
	} else {
		logger.Info("Topic not synchronized before")
	}

	// Sync to Aiven; retry if necessary
	result := r.Process(topic, logger)

	defer func() {
		metrics.TopicsProcessed.With(prometheus.Labels{
			metrics.LabelSyncState: result.Status.SynchronizationState,
			metrics.LabelPool:      topic.Spec.Pool,
		}).Inc()
	}()

	if result.Skipped {
		return ctrl.Result{}, nil
	}

	if result.Error != nil {
		topic.Status = &result.Status
		err = r.Update(ctx, &topic)
		if err != nil {
			logger.Errorf("Write resource status: %s", err)
		}
		return fail(result.Error, result.Requeue)
	}

	// If Aiven was purged of data, mark resource as finally deleted by removing finalizer.
	// Otherwise, append Kafkarator to finalizers if data is to be removed when topic is deleted.
	if result.DeleteFinalized {
		topic.RemoveFinalizer()
	} else if topic.RemoveDataWhenDeleted() {
		topic.AppendFinalizer()
	}

	// Write topic status; retry always
	topic.Status = &result.Status
	err = r.Update(ctx, &topic)
	if err != nil {
		return fail(err, true)
	}

	logger.WithFields(
		log.Fields{
			LogFieldSynchronizationState: topic.Status.SynchronizationState,
		},
	).Infof("Topic object written back to Kubernetes: %s", topic.Status.Message)

	return ctrl.Result{}, nil
}

func (r *TopicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafka_nais_io_v1.Topic{}).
		Complete(r)
}
