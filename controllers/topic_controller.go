package controllers

import (
	"context"
	"fmt"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/liberator/pkg/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
func (r *TopicReconciler) Process(topic kafka_nais_io_v1.Topic, logger *log.Entry) TopicReconcileResult {
	var err error
	var hash string
	var status kafka_nais_io_v1.TopicStatus

	if topic.Status != nil {
		status = *topic.Status
	}

	status.FullyQualifiedName = topic.FullName()

	fail := func(err error, state controller.SynchronizationState, retry bool) TopicReconcileResult {
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
		if !retry {
			status.LatestAivenSyncFailure = time.Now().Format(time.RFC3339)
		}

		return TopicReconcileResult{
			Requeue: retry,
			Status:  status,
			Error:   fmt.Errorf("%s: %s", state.String(), err),
		}
	}

	// Process or delete?
	projectName := topic.Spec.Pool
	serviceName, err := r.Aiven.NameResolver.ResolveKafkaServiceName(projectName)
	if err != nil {
		return fail(err, controller.SynchronizationStateFailed, false)
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
		}
		err = aclManager.Synchronize()
		if err != nil {
			return fail(fmt.Errorf("failed to delete ACLs on Aiven: %s", err), controller.SynchronizationStateFailed, true)
		}
		status.Message = "Topic and ACLs deleted, data kept"

		if topic.RemoveDataWhenDeleted() {
			logger.Info("Permanently deleting Aiven topic and its data")
			err = r.Aiven.Topics.Delete(projectName, serviceName, topic.FullName())
			if err != nil {
				if aiven.IsNotFound(err) {
					logger.Info("Topic already removed from Aiven")
				} else {
					return fail(fmt.Errorf("failed to delete topic on Aiven: %s", err), controller.SynchronizationStateFailed, true)
				}
			}
			status.Message = "Topic, ACLs and data permanently deleted"
		}

		logger.Info(status.Message)
		status.SynchronizationTime = &metav1.Time{Time: time.Now()}
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
		return fail(fmt.Errorf("unable to calculate synchronization hash"), controller.SynchronizationStateFailed, false)
	}

	if !topic.NeedsSynchronization(hash) {
		logger.Info("Synchronization already complete")
		return TopicReconcileResult{
			Skipped: true,
		}
	}

	if !r.projectWhitelisted(projectName) {
		return fail(fmt.Errorf("pool '%s' cannot be used in this cluster", projectName), controller.SynchronizationStateFailed, false)
	}

	synchronizer, _ := NewSynchronizer(r.Aiven, topic, logger)
	err = synchronizer.Synchronize()
	if err != nil {
		return fail(err, controller.SynchronizationStateFailed, true)
	}

	status.SynchronizationTime = &metav1.Time{Time: time.Now()}
	status.SynchronizationState = controller.SynchronizationStateSuccessful
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

	if result.Skipped {
		return ctrl.Result{}, nil
	}

	defer func() {
		metrics.TopicsProcessed.With(prometheus.Labels{
			metrics.LabelSyncState: string(result.Status.SynchronizationState),
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
	// Otherwise, append Kafkarator to finalizers to ensure proper cleanup when topic is deleted
	if result.DeleteFinalized {
		controllerutil.RemoveFinalizer(&topic, Finalizer)
	} else {
		controllerutil.AddFinalizer(&topic, Finalizer)
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
