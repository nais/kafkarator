package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/api/v1"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/kafkarator/pkg/aiven/service"
	"github.com/nais/kafkarator/pkg/aiven/serviceuser"
	"github.com/nais/kafkarator/pkg/aiven/topic"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	requeueInterval          = 10 * time.Second
	KafkaBrokers             = "KAFKA_BROKERS"
	KafkaSchemaRegistry      = "KAFKA_SCHEMA_REGISTRY"
	KafkaCertificatePath     = "KAFKA_CERTIFICATE_PATH"
	KafkaPrivateKeyPath      = "KAFKA_PRIVATE_KEY_PATH"
	KafkaCAPath              = "KAFKA_CA_PATH"
	KafkaCertificate         = "KAFKA_CERTIFICATE"
	KafkaCertificateFilename = "/var/run/secrets/kafka/kafka.crt"
	KafkaPrivateKey          = "KAFKA_PRIVATE_KEY"
	KafkaPrivateKeyFilename  = "/var/run/secrets/kafka/kafka.key"
	KafkaCA                  = "KAFKA_CA"
	KafkaCAFilename          = "/var/run/secrets/kafka/ca.crt"
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

	tx.logger.Infof("Synchronizing secrets")
	serviceManager := service.Manager{
		AivenCA:      r.Aiven.CA,
		AivenService: r.Aiven.Services,
		Project:      tx.topic.Spec.Pool,
		Service:      aivenService(tx.topic.Spec.Pool),
		Logger:       tx.logger,
	}
	svc, err := serviceManager.Get()
	if err != nil {
		return err
	}
	kafkaBrokerAddress := service.GetKafkaBrokerAddress(*svc)
	kafkaSchemaRegistryAddress := service.GetSchemaRegistryAddress(*svc)
	kafkaCA, err := serviceManager.GetCA()
	if err != nil {
		return err
	}

	for _, user := range users {
		secret := ConvertSecret(*user, tx.topic.Spec.Pool, kafkaBrokerAddress, kafkaSchemaRegistryAddress, kafkaCA)
		err = r.Create(tx.ctx, &secret)
		if err != nil {
			return err
		}
	}

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

func ConvertSecret(user aiven.ServiceUser, pool, brokers, registry, ca string) v1.Secret {
	return v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pool + "-kafka",
			Namespace: user.Username,
			Labels: map[string]string{
				"team": user.Username,
			},
		},
		StringData: map[string]string{
			KafkaCertificate:     user.AccessCert,
			KafkaPrivateKey:      user.AccessKey,
			KafkaBrokers:         brokers,
			KafkaSchemaRegistry:  registry,
			KafkaCertificatePath: KafkaCertificateFilename,
			KafkaPrivateKeyPath:  KafkaPrivateKeyFilename,
			KafkaCAPath:          KafkaCAFilename,
			KafkaCA:              ca,
		},
		Type: v1.SecretTypeOpaque,
	}
}
