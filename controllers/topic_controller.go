package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/api/v1"
	"github.com/nais/kafkarator/pkg/aiven"
	"github.com/nais/kafkarator/pkg/certificate"
	"github.com/nais/kafkarator/pkg/crypto"
	"github.com/nais/kafkarator/pkg/kafka/producer"
	"github.com/nais/kafkarator/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	KafkaBrokers           = "KAFKA_BROKERS"
	KafkaSchemaRegistry    = "KAFKA_SCHEMA_REGISTRY"
	KafkaCertificate       = "KAFKA_CERTIFICATE"
	KafkaPrivateKey        = "KAFKA_PRIVATE_KEY"
	KafkaCA                = "KAFKA_CA"
	KafkaCredStorePassword = "KAFKA_CREDSTORE_PASSWORD"
	KafkaKeystore          = "client.keystore.p12"
	KafkaTruststore        = "client.truststore.jks"
	maxSecretNameLength    = 63
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
	CryptManager        crypto.Manager
	Aiven               kafkarator_aiven.Interfaces
	Logger              *log.Logger
	Producer            producer.Interface
	Projects            []string
	RequeueInterval     time.Duration
	CredentialsLifetime time.Duration
	StoreGenerator      certificate.Generator
}

func (r *TopicReconciler) projectWhitelisted(project string) bool {
	for _, p := range r.Projects {
		if p == project {
			return true
		}
	}
	return false
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

	fail := func(err error, state string, retry bool) (ctrl.Result, error) {
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
		topicResource.Status.SynchronizationState = state

		logger.Errorf("%s: %s", state, err)

		if retry {
			return ctrl.Result{
				RequeueAfter: r.RequeueInterval,
			}, nil
		} else {
			return ctrl.Result{}, nil
		}
	}

	defer func() {
		logger.Infof("Finished processing request")
	}()

	// purge other systems if resource was deleted
	err := r.Get(ctx, req.NamespacedName, &topicResource)
	switch {
	case errors.IsNotFound(err):
		logger.Errorf("Resource deleted from cluster; noop")
		return ctrl.Result{}, nil
	case err != nil:
		logger.Errorf("Unable to retrieve resource from cluster: %s", err)
		return ctrl.Result{
			RequeueAfter: r.RequeueInterval,
		}, nil
	}

	logger = logger.WithFields(log.Fields{
		"team":          topicResource.Labels["team"],
		"aiven_project": topicResource.Spec.Pool,
		"aiven_topic":   topicResource.FullName(),
	})

	if topicResource.Status == nil {
		topicResource.Status = &kafka_nais_io_v1.TopicStatus{}
	}

	hash, err = topicResource.Spec.Hash()
	if err != nil {
		logger.Errorf("Unable to calculate synchronization hash")
		return ctrl.Result{}, err
	}

	if !topicResource.NeedsSynchronization(hash) {
		logger.Infof("Synchronization already complete")
		return ctrl.Result{}, nil
	}

	// Processing starts here
	if !r.projectWhitelisted(topicResource.Spec.Pool) {
		return fail(fmt.Errorf("pool '%s' cannot be used in this cluster", topicResource.Spec.Pool), kafka_nais_io_v1.EventFailedPrepare, false)
	}

	// Update Topic resource with status event
	defer func() {
		topicResource.Status.SynchronizationTime = time.Now().Format(time.RFC3339)
		err := r.Update(ctx, &topicResource)
		if err != nil {
			logger.Errorf("failed writing topic status: %s", err)
		}
		metrics.TopicsProcessed.With(prometheus.Labels{
			metrics.LabelSyncState: topicResource.Status.SynchronizationState,
			metrics.LabelPool:      topicResource.Spec.Pool,
		}).Inc()
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
		return fail(err, kafka_nais_io_v1.EventFailedSynchronization, true)
	}

	credentialsThreshold := time.Now().Add(r.CredentialsLifetime)

	topicResource.Status.SynchronizationState = kafka_nais_io_v1.EventRolloutComplete
	topicResource.Status.SynchronizationHash = hash
	topicResource.Status.CredentialsExpiryTime = credentialsThreshold.Format(time.RFC3339)
	topicResource.Status.Message = "Topic configuration synchronized to Kafka pool"
	topicResource.Status.Errors = nil

	return ctrl.Result{}, nil
}

func (r *TopicReconciler) commit(tx transaction) error {
	synchronizer := NewSynchronizer(r.Aiven, tx.topic, tx.logger)
	result, err := synchronizer.Synchronize(tx.topic)
	if err != nil {
		return err
	}

	tx.logger.Infof("Producing secrets")
	for _, user := range result.users {
		secret, err := Secret(tx.topic, r.StoreGenerator, *user, result.brokers, result.registry, result.ca)
		if err != nil {
			return err
		}

		tx.logger = tx.logger.WithFields(log.Fields{
			"username":         user.Username,
			"secret_namespace": secret.Namespace,
			"secret_name":      secret.Name,
		})

		plaintext, err := secret.Marshal()
		if err != nil {
			return fmt.Errorf("unable to marshal secret: %w", err)
		}

		ciphertext, err := r.CryptManager.Encrypt(plaintext)
		if err != nil {
			return fmt.Errorf("unable to encrypt secret: %w", err)
		}

		offset, err := r.Producer.Produce(ciphertext)
		if err != nil {
			return fmt.Errorf("unable to produce secret on Kafka: %w", err)
		}

		tx.logger.WithFields(log.Fields{
			"kafka_offset": offset,
		}).Infof("Produced secret")
	}

	return nil
}

func (r *TopicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafka_nais_io_v1.Topic{}).
		Complete(r)
}
