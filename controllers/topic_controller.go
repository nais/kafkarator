package controllers

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	kafka_nais_io_v1 "github.com/nais/kafkarator/api/v1"
	"github.com/nais/kafkarator/pkg/aiven"

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
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kafka.nais.io,resources=topics,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.nais.io,resources=topics/status,verbs=get;update;patch

func (r *TopicReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	hash := ""
	var topic kafka_nais_io_v1.Topic

	// purge other systems if resource was deleted
	err := r.Get(ctx, req.NamespacedName, &topic)
	switch {
	case errors.IsNotFound(err):
		// TODO: Handle this?
	case err != nil:
		// TODO: r.logger.Error(err, "unable to get jwker resource from cluster")
		return ctrl.Result{
			RequeueAfter: requeueInterval,
		}, nil
	}

	if topic.Status == nil {
		topic.Status = &kafka_nais_io_v1.TopicStatus{}
	}

	hash, err = topic.Spec.Hash()
	if err != nil {
		return ctrl.Result{}, err
	}
	if topic.Status.SynchronizationHash == hash {
		return ctrl.Result{}, nil
	}

	// Update Topic resource with status event
	defer func() {
		topic.Status.SynchronizationTime = time.Now().Format(time.RFC3339)
		err := r.Update(ctx, &topic)
		if err != nil {
			// TODO: r.logger.Error(err, "failed writing status")
		}
	}()

	// prepare and commit
	tx, err := r.prepare(ctx, req)
	if err != nil {
		topic.Status.SynchronizationState = kafka_nais_io_v1.EventFailedPrepare
		// TODO: r.logger.Error(err, "failed prepare jwks")
		return ctrl.Result{
			RequeueAfter: requeueInterval,
		}, nil
	}

	tx.topic = topic
	err = r.create(*tx)
	if err != nil {
		topic.Status.SynchronizationState = kafka_nais_io_v1.EventFailedSynchronization
		// TODO: r.logger.Error(err, "failed synchronization")
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
	aiven_client := &aiven.Client{
		Token:   os.Getenv("AIVEN_TOKEN"),
		Project: "nav-integration-test",
		Service: "nav-integration-test-kafka",
	}

	retention, err := strconv.Atoi(tx.topic.Spec.Config["retention.ms"])
	if err != nil {
		return fmt.Errorf("could not convert retention %s to number: %s", tx.topic.Spec.Config["retention.ms"], err)
	}

	partitions, err := strconv.Atoi(tx.topic.Spec.Config["kafka.partitions"])
	if err != nil {
		return fmt.Errorf("could not convert kafka partitions %s to number: %s", tx.topic.Spec.Config["kafka.partitions"], err)
	}

	replication, err := strconv.Atoi(tx.topic.Spec.Config["replication"])
	if err != nil {
		return fmt.Errorf("could not convert replication %s to number: %s", tx.topic.Spec.Config["replication"], err)
	}

	payload := aiven.CreateTopicRequest{
		Config: aiven.Config{
			"retention_ms": retention,
		},
		TopicName:   tx.topic.Name,
		Partitions:  partitions,
		Replication: replication,
	}
	return aiven_client.CreateTopic(payload)
}

func (r *TopicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafka_nais_io_v1.Topic{}).
		Complete(r)
}
