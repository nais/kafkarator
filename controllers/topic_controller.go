package controllers

import (
	"context"
	"errors"
	"fmt"
	"github.com/nais/kafkarator/pkg/utils"
	"net/http"
	"time"

	"github.com/aiven/aiven-go-client/v2"
	kafkarator_aiven "github.com/nais/kafkarator/pkg/aiven"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/kafkarator/pkg/metrics"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apimachinery_errors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	LogFieldSynchronizationState = "synchronization_state"
)

type TopicReconcileResult struct {
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
	DryRun          bool
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
func (r *TopicReconciler) Process(ctx context.Context, topic kafka_nais_io_v1.Topic, logger *log.Entry) TopicReconcileResult {
	var err error
	var hash string
	var status kafka_nais_io_v1.TopicStatus

	if topic.Status != nil {
		status = *topic.Status
	}

	status.FullyQualifiedName = topic.FullName()

	fail := func(err error, state string, retry bool) TopicReconcileResult {
		var aivenError aiven.Error
		propagatedErr := err
		ok := errors.As(err, &aivenError)
		if !ok {
			status.Message = err.Error()
			status.Errors = []string{
				err.Error(),
			}
		} else {
			// In rare cases, the Aiven client can return an error with StatusOK.
			// In these cases, the actual content of the error is not really relevant, because it is simply the response body
			// while the error was something related to I/O.
			// Since the response body may contain sensitive information, we do not want to log the message in this situation.
			if aivenError.Status == http.StatusOK {
				status.Message = "unknown error while calling Aiven API"
				status.Errors = []string{
					status.Message,
				}
				propagatedErr = errors.New(status.Message)
			} else {
				status.Message = aivenError.Message
				status.Errors = []string{
					fmt.Sprintf("%s: %s", aivenError.Message, aivenError.MoreInfo),
				}
			}
		}
		status.SynchronizationState = state
		if !retry {
			status.LatestAivenSyncFailure = time.Now().Format(time.RFC3339)
		}

		propagatedErr = utils.CheckForPossibleCredentials(propagatedErr)

		return TopicReconcileResult{
			Requeue: retry,
			Status:  status,
			Error:   fmt.Errorf("%s: %s", state, propagatedErr),
		}
	}

	// Process or delete?
	projectName := topic.Spec.Pool
	serviceName, err := r.Aiven.NameResolver.ResolveKafkaServiceName(ctx, projectName)
	if err != nil {
		return fail(err, kafka_nais_io_v1.EventFailedSynchronization, false)
	}

	if topic.ObjectMeta.DeletionTimestamp != nil {
		logger.Info("Deleting ACls for topic")
		strippedTopic := topic.DeepCopy()
		strippedTopic.Spec.ACL = nil
		aclManager := acl.Manager{
			AivenACLs: r.Aiven.ACLs,
			Project:   projectName,
			Service:   serviceName,
			Source:    acl.TopicAdapter{Topic: strippedTopic},
			Logger:    logger,
			DryRun:    r.DryRun,
		}
		err = aclManager.Synchronize(ctx)
		if err != nil {
			return fail(fmt.Errorf("failed to delete ACLs on Aiven: %s", err), kafka_nais_io_v1.EventFailedSynchronization, true)
		}
		status.Message = "Topic and ACLs deleted, data kept"

		if topic.RemoveDataWhenDeleted() {
			logger.Info("Permanently deleting Aiven topic and its data")
			err = metrics.ObserveAivenLatency("Topic_Delete", projectName, func() error {
				if r.DryRun {
					r.Logger.Infof("DRY RUN: Would delete Topic: %v", topic.FullName())
					return nil
				}
				return r.Aiven.Topics.Delete(ctx, projectName, serviceName, topic.FullName())
			})
			if err != nil {
				if aiven.IsNotFound(err) {
					logger.Info("Topic already removed from Aiven")
				} else {
					return fail(fmt.Errorf("failed to delete topic on Aiven: %s", err), kafka_nais_io_v1.EventFailedSynchronization, true)
				}
			}
			status.Message = "Topic, ACLs and data permanently deleted"
		}

		logger.Info(status.Message)
		status.SynchronizationTime = time.Now().Format(time.RFC3339)
		status.Errors = nil
		return TopicReconcileResult{
			DeleteFinalized: true,
			Status:          status,
		}
	}

	if topic.Spec.Config == nil {
		topic.Spec.Config = &kafka_nais_io_v1.Config{}
	}
	topic.Spec.Config.ApplyDefaults()

	hash, err = topic.Hash()
	if err != nil {
		return fail(fmt.Errorf("unable to calculate synchronization hash"), kafka_nais_io_v1.EventFailedPrepare, false)
	}

	if !topic.NeedsSynchronization(hash) {
		logger.Info("Synchronization already complete")
		return TopicReconcileResult{
			Skipped: true,
		}
	}

	if !r.projectWhitelisted(projectName) {
		return fail(fmt.Errorf("pool '%s' cannot be used in this cluster", projectName), kafka_nais_io_v1.EventFailedPrepare, false)
	}

	synchronizer, err := NewSynchronizer(ctx, r.Aiven, topic, logger, r.DryRun)
	if err != nil {
		return fail(err, kafka_nais_io_v1.EventFailedSynchronization, false)
	}
	err = synchronizer.Synchronize(ctx)
	if err != nil {
		return fail(err, kafka_nais_io_v1.EventFailedSynchronization, true)
	}

	status.SynchronizationTime = time.Now().Format(time.RFC3339)
	status.SynchronizationState = kafka_nais_io_v1.EventRolloutComplete
	status.SynchronizationHash = hash
	status.Message = "Topic configuration synchronized to Kafka pool"
	status.Errors = nil
	status.LatestAivenSyncFailure = ""

	return TopicReconcileResult{
		Status: status,
	}
}

// +kubebuilder:rbac:groups=kafka.nais.io,resources=topics,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.nais.io,resources=topics/status,verbs=get;update;patch

func (r *TopicReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
	case apimachinery_errors.IsNotFound(err):
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

	// Append Kafkarator to finalizers to ensure proper cleanup when topic is deleted
	controllerutil.AddFinalizer(&topic, Finalizer)

	// Sync to Aiven; retry if necessary
	result := r.Process(ctx, topic, logger)

	if result.Skipped {
		return ctrl.Result{}, nil
	}

	defer func() {
		metrics.TopicsProcessed.With(prometheus.Labels{
			metrics.LabelSyncState: result.Status.SynchronizationState,
			metrics.LabelPool:      topic.Spec.Pool,
		}).Inc()
	}()

	if result.Error != nil {
		topic.Status = &result.Status
		err = r.Update(ctx, &topic)
		if err != nil {
			logger.Errorf("Write resource status: %s", err)
		}
		return fail(result.Error, result.Requeue)
	}

	// If Aiven was purged of data, mark resource as finally deleted by removing finalizer.
	if result.DeleteFinalized {
		controllerutil.RemoveFinalizer(&topic, Finalizer)
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
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 10,
		}).
		For(&kafka_nais_io_v1.Topic{}).
		Complete(r)
}
