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
	v1 "k8s.io/api/core/v1"
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

	maxSecretNameLength = 63
)

type ReconcileResult struct {
	DeleteFinalized bool
	Skipped         bool
	Requeue         bool
	Status          kafka_nais_io_v1.TopicStatus
	Secrets         []v1.Secret
	Error           error
}

type TopicReconciler struct {
	client.Client
	Aiven               kafkarator_aiven.Interfaces
	CredentialsLifetime time.Duration
	CryptManager        crypto.Manager
	Logger              *log.Logger
	Producer            producer.Interface
	Projects            []string
	RequeueInterval     time.Duration
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

// Write changes to Aiven and return a topic processing status and a list of secrets to send to clusters
func (r *TopicReconciler) Process(topic kafka_nais_io_v1.Topic, logger *log.Entry) ReconcileResult {
	var err error
	var hash string
	var status kafka_nais_io_v1.TopicStatus

	if topic.Status != nil {
		status = *topic.Status
	}

	fail := func(err error, state string, retry bool) ReconcileResult {
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

		return ReconcileResult{
			Requeue: retry,
			Status:  status,
			Error:   fmt.Errorf("%s: %s", state, err),
		}
	}

	// Process or delete?
	if topic.ObjectMeta.DeletionTimestamp != nil {
		if topic.RemoveDataWhenDeleted() {
			logger.Infof("Permanently deleting Aiven topic and its data")
			err = r.Aiven.Topics.Delete(topic.Spec.Pool, kafkarator_aiven.ServiceName(topic.Spec.Pool), topic.FullName())
			if err != nil {
				return fail(fmt.Errorf("failed to delete topic on Aiven: %s", err), kafka_nais_io_v1.EventFailedSynchronization, true)
			}
			status.Message = "Topic and data permanently deleted"
		}
		status.SynchronizationTime = time.Now().Format(time.RFC3339)
		status.Errors = nil
		return ReconcileResult{
			DeleteFinalized: true,
			Status:          status,
		}
	}

	hash, err = topic.Hash()
	if err != nil {
		return fail(fmt.Errorf("unable to calculate synchronization hash"), kafka_nais_io_v1.EventFailedPrepare, false)
	}

	if !topic.NeedsSynchronization(hash) {
		logger.Infof("Synchronization already complete")
		return ReconcileResult{
			Skipped: true,
		}
	}

	if !r.projectWhitelisted(topic.Spec.Pool) {
		return fail(fmt.Errorf("pool '%s' cannot be used in this cluster", topic.Spec.Pool), kafka_nais_io_v1.EventFailedPrepare, false)
	}

	synchronizer := NewSynchronizer(r.Aiven, topic, logger)
	result, err := synchronizer.Synchronize(topic)
	if err != nil {
		return fail(err, kafka_nais_io_v1.EventFailedSynchronization, true)
	}

	secrets := make([]v1.Secret, len(result.users))
	for i, user := range result.users {
		secret, err := Secret(topic, r.StoreGenerator, *user, result.brokers, result.registry, result.ca)
		if err != nil {
			return fail(err, kafka_nais_io_v1.EventFailedPrepare, false)
		}
		secrets[i] = *secret
	}

	status.SynchronizationTime = time.Now().Format(time.RFC3339)
	status.SynchronizationState = kafka_nais_io_v1.EventRolloutComplete
	status.SynchronizationHash = hash
	status.CredentialsExpiryTime = time.Now().Add(r.CredentialsLifetime).Format(time.RFC3339)
	status.Message = "Topic configuration synchronized to Kafka pool"
	status.Errors = nil

	return ReconcileResult{
		Status:  status,
		Secrets: secrets,
	}
}

func (r *TopicReconciler) produceSecret(logger *log.Entry, secret v1.Secret) error {
	logger.Infof("Producing secrets")
	logger = logger.WithFields(log.Fields{
		// "username":         user.Username,
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

	logger.WithFields(log.Fields{
		"kafka_offset": offset,
	}).Infof("Produced secret")

	return nil
}

// +kubebuilder:rbac:groups=kafka.nais.io,resources=topics,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.nais.io,resources=topics/status,verbs=get;update;patch

func (r *TopicReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
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

	ctx := context.Background()

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

	// Sync to Aiven; retry if necessary
	result := r.Process(topic, logger)

	defer func() {
		metrics.TopicsProcessed.With(prometheus.Labels{
			metrics.LabelSyncState: result.Status.SynchronizationState,
			metrics.LabelPool:      topic.Spec.Pool,
		}).Inc()
	}()

	if result.Skipped {
		return ctrl.Result{}, nil
	}

	if result.Error != nil {
		return fail(err, result.Requeue)
	}

	// If Aiven was purged of data, mark resource as finally deleted by removing finalizer.
	// Otherwise, append Kafkarator to finalizers if data is to be removed when topic is deleted.
	if result.DeleteFinalized {
		topic.RemoveFinalizer()
	} else if topic.RemoveDataWhenDeleted() {
		topic.AppendFinalizer()
	}

	// Produce secrets; retry always
	for _, secret := range result.Secrets {
		err = r.produceSecret(logger, secret)
		if err != nil {
			return fail(err, true)
		}
	}

	// Write topic status; retry always
	topic.Status = &result.Status
	err = r.Update(ctx, &topic)
	if err != nil {
		return fail(err, true)
	}

	return ctrl.Result{}, nil
}

func (r *TopicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafka_nais_io_v1.Topic{}).
		Complete(r)
}
