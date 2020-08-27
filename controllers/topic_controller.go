package controllers

import (
	"context"
	"time"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/api/v1"
	log "github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	requeueInterval = 10 * time.Second
)

type transaction struct {
	ctx   context.Context
	req   ctrl.Request
	topic kafka_nais_io_v1.Topic
}

type TopicReconciler struct {
	client.Client
	Aiven  *aiven.Client
	Scheme *runtime.Scheme
}

func aivenService(aivenProject string) string {
	return aivenProject + "-kafka"
}

func intp(i int) *int {
	return &i
}

// +kubebuilder:rbac:groups=kafka.nais.io,resources=topics,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.nais.io,resources=topics/status,verbs=get;update;patch

func (r *TopicReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log.Infof("Processing request for %s", req.Name)

	ctx := context.Background()
	hash := ""
	var topic kafka_nais_io_v1.Topic

	defer func() {
		log.Infof("Finished processing request for %s", req.Name)
	}()

	// purge other systems if resource was deleted
	err := r.Get(ctx, req.NamespacedName, &topic)
	switch {
	case errors.IsNotFound(err):
		log.Errorf("Resource not found in cluster")
	case err != nil:
		log.Errorf("Unable to retrieve resource from cluster: %s", err)
		return ctrl.Result{
			RequeueAfter: requeueInterval,
		}, nil
	}

	if topic.Status == nil {
		topic.Status = &kafka_nais_io_v1.TopicStatus{}
	}

	hash, err = topic.Spec.Hash()
	if err != nil {
		log.Errorf("Unable to calculate synchronization hash")
		return ctrl.Result{}, err
	}
	if topic.Status.SynchronizationHash == hash {
		log.Infof("Synchronization already complete")
		return ctrl.Result{}, nil
	}

	// Update Topic resource with status event
	defer func() {
		topic.Status.SynchronizationTime = time.Now().Format(time.RFC3339)
		err := r.Update(ctx, &topic)
		if err != nil {
			log.Errorf("failed writing topic status: %s", err)
		}
	}()

	// prepare and commit
	tx, err := r.prepare(ctx, req)
	if err != nil {
		topic.Status.SynchronizationState = kafka_nais_io_v1.EventFailedPrepare
		log.Errorf("failed preparing transaction: %s", err)
		return ctrl.Result{
			RequeueAfter: requeueInterval,
		}, nil
	}

	tx.topic = topic
	err = r.create(*tx)
	if err != nil {
		topic.Status.SynchronizationState = kafka_nais_io_v1.EventFailedSynchronization
		topic.Status.Message = err.Error()
		topic.Status.Errors = []string{
			err.Error(),
		}
		log.Errorf("failed synchronization: %s", err)
		return ctrl.Result{
			RequeueAfter: requeueInterval,
		}, nil
	}

	topic.Status.SynchronizationState = kafka_nais_io_v1.EventRolloutComplete
	topic.Status.SynchronizationHash = hash

	// TODO: delete unused secrets from cluster
	/*
		for _, oldSecret := range tx.secretLists.Unused.Items {
			err = r.Delete(tx.ctx, &oldSecret)
			r.logger.Error(err, "failed deletion")
		}
	*/

	return ctrl.Result{}, nil
}

func (r *TopicReconciler) prepare(ctx context.Context, req ctrl.Request) (*transaction, error) {
	return &transaction{
		ctx: ctx,
		req: req,
	}, nil
}

func (r *TopicReconciler) create(tx transaction) error {
	cfg := tx.topic.Spec.Config
	if cfg == nil {
		cfg = &kafka_nais_io_v1.Config{}
	}

	req := aiven.CreateKafkaTopicRequest{
		CleanupPolicy:         cfg.CleanupPolicy,
		MinimumInSyncReplicas: cfg.MinimumInSyncReplicas,
		Partitions:            cfg.Partitions,
		Replication:           cfg.Replication,
		RetentionBytes:        cfg.RetentionBytes,
		RetentionHours:        cfg.RetentionHours,
		TopicName:             tx.topic.Name,
	}

	return r.Aiven.KafkaTopics.Create(tx.topic.Spec.Pool, aivenService(tx.topic.Spec.Pool), req)
}

func (r *TopicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafka_nais_io_v1.Topic{}).
		Complete(r)
}
