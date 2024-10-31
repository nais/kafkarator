package controllers

import (
	"context"
	"fmt"
	"github.com/aiven/aiven-go-client/v2"
	kafkarator_aiven "github.com/nais/kafkarator/pkg/aiven"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/kafkarator/pkg/metrics"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
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
	result := r.Process(ctx, stream, logger)

	if result.Skipped {
		return ctrl.Result{}, nil
	}

	defer func() {
		metrics.StreamsProcessed.With(prometheus.Labels{
			metrics.LabelSyncState: result.Status.SynchronizationState,
			metrics.LabelPool:      stream.Spec.Pool,
		}).Inc()
	}()

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
		controllerutil.RemoveFinalizer(&stream, Finalizer)
	} else {
		controllerutil.AddFinalizer(&stream, Finalizer)
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

func (r *StreamReconciler) Process(ctx context.Context, stream kafka_nais_io_v1.Stream, logger log.FieldLogger) StreamReconcileResult {
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
		return r.handleDelete(ctx, stream, logger, status, fail)
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

	projectName := stream.Spec.Pool
	if !r.projectWhitelisted(projectName) {
		return fail(fmt.Errorf("pool '%s' cannot be used in this cluster", projectName), kafka_nais_io_v1.EventFailedPrepare, false)
	}

	serviceName, err := r.Aiven.NameResolver.ResolveKafkaServiceName(ctx, projectName)
	if err != nil {
		return fail(err, kafka_nais_io_v1.EventFailedSynchronization, false)
	}
	aclManager := acl.Manager{
		AivenACLs: r.Aiven.ACLs,
		Project:   projectName,
		Service:   serviceName,
		Source:    acl.StreamAdapter{Stream: &stream},
		Logger:    logger,
	}
	err = aclManager.Synchronize(ctx)
	if err != nil {
		return fail(err, kafka_nais_io_v1.EventFailedSynchronization, true)
	}

	status.SynchronizationTime = time.Now().Format(time.RFC3339)
	status.SynchronizationState = kafka_nais_io_v1.EventRolloutComplete
	status.SynchronizationHash = hash
	status.Message = "Stream configuration synchronized to Kafka pool"
	status.Errors = nil

	return StreamReconcileResult{
		Status: status,
	}
}

func (r *StreamReconciler) handleDelete(ctx context.Context, stream kafka_nais_io_v1.Stream, logger log.FieldLogger, status kafka_nais_io_v1.StreamStatus, fail func(err error, state string, retry bool) StreamReconcileResult) StreamReconcileResult {
	logger.Infof("Permanently deleting Aiven stream topics, ACLs and its data")

	projectName := stream.Spec.Pool
	if !r.projectWhitelisted(projectName) {
		return fail(fmt.Errorf("pool '%s' cannot be used in this cluster", projectName), kafka_nais_io_v1.EventFailedPrepare, false)
	}

	serviceName, err := r.Aiven.NameResolver.ResolveKafkaServiceName(ctx, projectName)
	if err != nil {
		return fail(err, kafka_nais_io_v1.EventFailedSynchronization, false)
	}

	aclManager := acl.Manager{
		AivenACLs: r.Aiven.ACLs,
		Project:   projectName,
		Service:   serviceName,
		Source:    acl.StreamAdapter{Stream: &stream, Delete: true},
		Logger:    logger,
	}
	err = aclManager.Synchronize(ctx)
	if err != nil {
		return fail(fmt.Errorf("failed to delete ACL %s on Aiven: %s", stream.ACL(), err), kafka_nais_io_v1.EventFailedSynchronization, true)
	}
	status.Message = "Deleted Stream ACL"

	logger.Infof("Permanently deleting Aiven stream and its data")
	topics, err := r.Aiven.Topics.List(ctx, projectName, serviceName)
	for _, topic := range topics {
		if strings.HasPrefix(topic.TopicName, stream.TopicPrefix()) {
			err = r.Aiven.Topics.Delete(ctx, projectName, serviceName, topic.TopicName)
			if err != nil {
				return fail(fmt.Errorf("failed to delete topic '%s' on Aiven: %s", topic.TopicName, err), kafka_nais_io_v1.EventFailedSynchronization, true)
			}
		}
	}
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
