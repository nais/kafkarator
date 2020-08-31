package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/api/v1"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/kafkarator/pkg/aiven/serviceuser"
	"github.com/nais/kafkarator/pkg/aiven/topic"
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
	ctx        context.Context
	req        ctrl.Request
	topic      kafka_nais_io_v1.Topic
	aivenTopic *aiven.KafkaTopic
	logger     *log.Entry
}

type TopicReconciler struct {
	client.Client
	Aiven  *aiven.Client
	Scheme *runtime.Scheme
	Logger *log.Logger
}

func aivenService(aivenProject string) string {
	return aivenProject + "-kafka"
}

// +kubebuilder:rbac:groups=kafka.nais.io,resources=topics,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.nais.io,resources=topics/status,verbs=get;update;patch

func (r *TopicReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	logger := log.NewEntry(r.Logger)

	logger = logger.WithFields(log.Fields{
		"topic":     req.Name,
		"namespace": req.Namespace,
	})

	logger.Infof("Processing request")

	ctx := context.Background()
	hash := ""

	var topicResource kafka_nais_io_v1.Topic

	defer func() {
		logger.Infof("Finished processing request")
	}()

	// purge other systems if resource was deleted
	err := r.Get(ctx, req.NamespacedName, &topicResource)
	switch {
	case errors.IsNotFound(err):
		logger.Errorf("Resource not found in cluster")
	case err != nil:
		logger.Errorf("Unable to retrieve resource from cluster: %s", err)
		return ctrl.Result{
			RequeueAfter: requeueInterval,
		}, nil
	}

	logger = logger.WithFields(log.Fields{
		"team": topicResource.Labels["team"],
	})

	if topicResource.Status == nil {
		topicResource.Status = &kafka_nais_io_v1.TopicStatus{}
	}

	hash, err = topicResource.Spec.Hash()
	if err != nil {
		logger.Errorf("Unable to calculate synchronization hash")
		return ctrl.Result{}, err
	}
	if topicResource.Status.SynchronizationHash == hash {
		logger.Infof("Synchronization already complete")
		return ctrl.Result{}, nil
	}

	// Update Topic resource with status event
	defer func() {
		topicResource.Status.SynchronizationTime = time.Now().Format(time.RFC3339)
		err := r.Update(ctx, &topicResource)
		if err != nil {
			logger.Errorf("failed writing topic status: %s", err)
		}
	}()

	// prepare and commit
	tx := &transaction{
		ctx:    ctx,
		req:    req,
		topic:  topicResource,
		logger: logger,
	}

	err = r.commit(*tx)

	if err != nil {
		aivenError, ok := err.(aiven.Error)
		if !ok {
			topicResource.Status.Message = err.Error()
			topicResource.Status.Errors = []string{
				err.Error(),
			}
		} else {
			topicResource.Status.Message = aivenError.Message
			topicResource.Status.Errors = []string{
				fmt.Sprintf("%s: %s", aivenError.Message, aivenError.MoreInfo),
			}
		}
		topicResource.Status.SynchronizationState = kafka_nais_io_v1.EventFailedSynchronization

		logger.Errorf("Failed synchronization: %s", err)

		return ctrl.Result{
			RequeueAfter: requeueInterval,
		}, nil
	}

	topicResource.Status.SynchronizationState = kafka_nais_io_v1.EventRolloutComplete
	topicResource.Status.SynchronizationHash = hash
	topicResource.Status.Message = "Topic configuration synchronized to Kafka pool"
	topicResource.Status.Errors = nil

	// TODO: delete unused secrets from cluster
	/*
		for _, oldSecret := range tx.secretLists.Unused.Items {
			err = r.Delete(tx.ctx, &oldSecret)
			r.logger.Error(err, "failed deletion")
		}
	*/

	return ctrl.Result{}, nil
}

func (r *TopicReconciler) commit(tx transaction) error {
	aclManager := acl.Manager{
		Aiven:   r.Aiven,
		Project: tx.topic.Spec.Pool,
		Service: aivenService(tx.topic.Spec.Pool),
		Topic:   tx.topic,
		Logger:  tx.logger,
	}

	tx.logger.Infof("Synchronizing access control lists")
	usernames, err := aclManager.Synchronize()
	if err != nil {
		return err
	}

	userManager := serviceuser.Manager{
		AivenServiceUsers: r.Aiven.ServiceUsers,
		Project:           tx.topic.Spec.Pool,
		Service:           aivenService(tx.topic.Spec.Pool),
		Logger:            tx.logger,
	}

	tx.logger.Infof("Synchronizing service users")
	users, err := userManager.Synchronize(usernames)
	if err != nil {
		return err
	}

	_ = users // TODO: create secrets

	tx.logger.Infof("Synchronizing topic")
	topicManager := topic.Manager{
		AivenTopics: r.Aiven.KafkaTopics,
		Project:     tx.topic.Spec.Pool,
		Service:     aivenService(tx.topic.Spec.Pool),
		Topic:       tx.topic,
		Logger:      tx.logger,
	}

	err = topicManager.Synchronize()

	return err
}

func (r *TopicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafka_nais_io_v1.Topic{}).
		Complete(r)
}
