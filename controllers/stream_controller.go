package controllers

import (
	"context"
	"fmt"
	"github.com/aiven/aiven-go-client"
	kafkarator_aiven "github.com/nais/kafkarator/pkg/aiven"
	"github.com/nais/kafkarator/pkg/metrics"
	"github.com/nais/kafkarator/pkg/utils"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type StreamReconcileResult struct {
	DeleteFinalized bool
	Skipped         bool
	Requeue         bool
	Status          kafka_nais_io_v1.StreamStatus
	Error           error
}

type StreamReconciler struct {
	client.Client
	Aiven           kafkarator_aiven.Interfaces
	Logger          log.FieldLogger
	Projects        []string
	RequeueInterval time.Duration
}

func (r *StreamReconciler) projectWhitelisted(project string) bool {
	for _, p := range r.Projects {
		if p == project {
			return true
		}
	}
	return false
}

// +kubebuilder:rbac:groups=kafka.nais.io,resources=streams,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.nais.io,resources=streams/status,verbs=get;update;patch
func (r *StreamReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var stream kafka_nais_io_v1.Stream

	logger := r.Logger.WithFields(log.Fields{
		"owner":     req.Name,
		"namespace": req.Namespace,
	})

	logger.Infof("Processing request")
	defer func() {
		logger.Infof("Finished processing request")
	}()

	fail := func(err error, requeue bool) (ctrl.Result, error) {
		logger.Error(err)
		cr := &ctrl.Result{}
		if requeue {
			cr.RequeueAfter = r.RequeueInterval
		}
		return *cr, nil
	}

	err := r.Get(ctx, req.NamespacedName, &stream)
	switch {
	case errors.IsNotFound(err):
		return fail(fmt.Errorf("resource deleted from cluster; noop"), false)
	case err != nil:
		return fail(fmt.Errorf("unable to retrieve resource from cluster: %s", err), true)
	}

	logger = logger.WithFields(log.Fields{
		"team":          stream.Labels["team"],
		"aiven_project": stream.Spec.Pool,
	})

	if stream.Status != nil {
		logger.Infof("Last synchronization time: %s", stream.Status.SynchronizationTime)
	} else {
		logger.Info("Stream not synchronized before")
	}

	// Sync to Aiven; retry if necessary
	result := r.Process(stream, logger)

	defer func() {
		metrics.StreamsProcessed.With(prometheus.Labels{
			metrics.LabelSyncState: result.Status.SynchronizationState,
			metrics.LabelPool:      stream.Spec.Pool,
		}).Inc()
	}()

	if result.Skipped {
		return ctrl.Result{}, nil
	}

	if result.Error != nil {
		stream.Status = &result.Status
		err = r.Update(ctx, &stream)
		if err != nil {
			logger.Errorf("Write resource status: %s", err)
		}
		return fail(result.Error, result.Requeue)
	}

	// If Aiven was purged of data, mark resource as finally deleted by removing finalizer.
	// Otherwise, append Kafkarator to finalizers to ensure proper cleanup when stream is deleted
	if result.DeleteFinalized {
		utils.RemoveFinalizer(&stream)
	} else {
		utils.AppendFinalizer(&stream)
	}

	// Write stream status; retry always
	stream.Status = &result.Status
	err = r.Update(ctx, &stream)
	if err != nil {
		return fail(err, true)
	}

	logger.WithFields(
		log.Fields{
			LogFieldSynchronizationState: stream.Status.SynchronizationState,
		},
	).Infof("Stream object written back to Kubernetes: %s", stream.Status.Message)

	return ctrl.Result{}, nil
}

func (r *StreamReconciler) Process(stream kafka_nais_io_v1.Stream, logger log.FieldLogger) StreamReconcileResult {
	var err error
	var hash string
	var status kafka_nais_io_v1.StreamStatus

	if stream.Status != nil {
		status = *stream.Status
	}

	status.FullyQualifiedTopicPrefix = stream.TopicPrefix()

	fail := func(err error, state string, retry bool) StreamReconcileResult {
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

		return StreamReconcileResult{
			Requeue: retry,
			Status:  status,
			Error:   fmt.Errorf("%s: %s", state, err),
		}
	}

	// Process or delete?
	if stream.ObjectMeta.DeletionTimestamp != nil {
		return r.handleDelete(logger, status)
	}

	hash, err = stream.Hash()
	if err != nil {
		return fail(fmt.Errorf("unable to calculate synchronization hash"), kafka_nais_io_v1.EventFailedPrepare, false)
	}

	if !stream.NeedsSynchronization(hash) {
		logger.Infof("Synchronization already complete")
		return StreamReconcileResult{
			Skipped: true,
		}
	}

	if !r.projectWhitelisted(stream.Spec.Pool) {
		return fail(fmt.Errorf("pool '%s' cannot be used in this cluster", stream.Spec.Pool), kafka_nais_io_v1.EventFailedPrepare, false)
	}

	// TODO: Sync stream ACL

	status.SynchronizationTime = time.Now().Format(time.RFC3339)
	status.SynchronizationState = kafka_nais_io_v1.EventRolloutComplete
	status.SynchronizationHash = hash
	status.Message = "Stream configuration synchronized to Kafka pool"
	status.Errors = nil

	return StreamReconcileResult{
		Status: status,
	}
}

func (r *StreamReconciler) handleDelete(logger log.FieldLogger, status kafka_nais_io_v1.StreamStatus) StreamReconcileResult {
	logger.Infof("Permanently deleting Aiven stream topics, ACLs and its data")
	// TODO: Delete ACL
	// TODO: Delete all stream topics
	status.Message = "Stream, ACLs and data permanently deleted"

	logger.Info(status.Message)
	status.SynchronizationTime = time.Now().Format(time.RFC3339)
	status.Errors = nil
	return StreamReconcileResult{
		DeleteFinalized: true,
		Status:          status,
	}
}

func (r *StreamReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafka_nais_io_v1.Stream{}).
		Complete(r)
}
