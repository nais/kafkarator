package controllers

import (
	"context"
	"fmt"
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
	ctx        context.Context
	req        ctrl.Request
	topic      kafka_nais_io_v1.Topic
	aivenTopic *aiven.KafkaTopic
	log        *log.Entry
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
	tx := &transaction{
		ctx:   ctx,
		req:   req,
		topic: topic,
		log:   log.NewEntry(log.StandardLogger()),
	}
	err = r.commit(*tx)
	if err != nil {
		aivenError, ok := err.(aiven.Error)
		if !ok {
			topic.Status.Message = err.Error()
			topic.Status.Errors = []string{
				err.Error(),
			}
		} else {
			topic.Status.Message = aivenError.Message
			topic.Status.Errors = []string{
				fmt.Sprintf("%s: %s", aivenError.Message, aivenError.MoreInfo),
			}
		}
		topic.Status.SynchronizationState = kafka_nais_io_v1.EventFailedSynchronization
		log.Errorf("failed synchronization: %s", err)
		return ctrl.Result{
			RequeueAfter: requeueInterval,
		}, nil
	}

	topic.Status.SynchronizationState = kafka_nais_io_v1.EventRolloutComplete
	topic.Status.SynchronizationHash = hash
	topic.Status.Message = "Topic configuration synchronized to Kafka pool"
	topic.Status.Errors = nil

	// TODO: delete unused secrets from cluster
	/*
		for _, oldSecret := range tx.secretLists.Unused.Items {
			err = r.Delete(tx.ctx, &oldSecret)
			r.logger.Error(err, "failed deletion")
		}
	*/

	return ctrl.Result{}, nil
}

func topicConfigChanged(topic *aiven.KafkaTopic, config *kafka_nais_io_v1.Config) bool {
	if config == nil {
		return false
	}
	if config.RetentionHours != nil && topic.RetentionHours != *config.RetentionHours {
		return true
	}
	if config.RetentionBytes != nil && topic.RetentionBytes != *config.RetentionBytes {
		return true
	}
	if config.Replication != nil && topic.Replication != *config.Replication {
		return true
	}
	if config.Partitions != nil && len(topic.Partitions) != *config.Partitions {
		return true
	}
	if config.MinimumInSyncReplicas != nil && topic.MinimumInSyncReplicas != *config.MinimumInSyncReplicas {
		return true
	}
	return false
}

func (r *TopicReconciler) commit(tx transaction) error {
	topic, err := r.Aiven.KafkaTopics.Get(tx.topic.Spec.Pool, aivenService(tx.topic.Spec.Pool), tx.topic.Name)
	if err == nil {
		// topic already exists
		if topicConfigChanged(topic, tx.topic.Spec.Config) {
			return r.update(tx)
		} else {
			tx.log.Infof("No changes for topic %s", tx.topic.Name)
			return nil
		}
	}

	return r.create(tx)
}

func (r *TopicReconciler) create(tx transaction) error {
	tx.log.Infof("Creating topic %s", tx.topic.Name)

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

func (r *TopicReconciler) update(tx transaction) error {
	tx.log.Infof("Updating topic %s", tx.topic.Name)

	cfg := tx.topic.Spec.Config
	if cfg == nil {
		cfg = &kafka_nais_io_v1.Config{}
	}

	req := aiven.UpdateKafkaTopicRequest{
		MinimumInSyncReplicas: cfg.MinimumInSyncReplicas,
		Partitions:            cfg.Partitions,
		Replication:           cfg.Replication,
		RetentionBytes:        cfg.RetentionBytes,
		RetentionHours:        cfg.RetentionHours,
	}

	return r.Aiven.KafkaTopics.Update(tx.topic.Spec.Pool, aivenService(tx.topic.Spec.Pool), tx.topic.Name, req)
}

func (r *TopicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafka_nais_io_v1.Topic{}).
		Complete(r)
}
