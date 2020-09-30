package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/api/v1"
	"github.com/nais/kafkarator/pkg/aiven"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/kafkarator/pkg/aiven/service"
	"github.com/nais/kafkarator/pkg/aiven/serviceuser"
	"github.com/nais/kafkarator/pkg/aiven/topic"
	"github.com/nais/kafkarator/pkg/crypto"
	"github.com/nais/kafkarator/pkg/kafka/producer"
	"github.com/nais/kafkarator/pkg/metrics"
	"github.com/nais/kafkarator/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	KafkaBrokers        = "KAFKA_BROKERS"
	KafkaSchemaRegistry = "KAFKA_SCHEMA_REGISTRY"
	KafkaCertificate    = "KAFKA_CERTIFICATE"
	KafkaPrivateKey     = "KAFKA_PRIVATE_KEY"
	KafkaCA             = "KAFKA_CA"
	maxSecretNameLength = 63
)

type transaction struct {
	ctx        context.Context
	req        ctrl.Request
	topic      kafka_nais_io_v1.Topic
	aivenTopic *aiven.KafkaTopic
	logger     *log.Entry
}

type secretData struct {
	user            aiven.ServiceUser
	resourceVersion string
	app             string
	pool            string
	name            string
	team            string
	brokers         string
	registry        string
	ca              string
}

type TopicReconciler struct {
	client.Client
	CryptManager    crypto.Manager
	Aiven           *aiven.Client
	Scheme          *runtime.Scheme
	Logger          *log.Logger
	Producer        *producer.Producer
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
	if topicResource.Status.SynchronizationHash == hash {
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

	topicResource.Status.SynchronizationState = kafka_nais_io_v1.EventRolloutComplete
	topicResource.Status.SynchronizationHash = hash
	topicResource.Status.Message = "Topic configuration synchronized to Kafka pool"
	topicResource.Status.Errors = nil

	return ctrl.Result{}, nil
}

func (r *TopicReconciler) commit(tx transaction) error {
	aclManager := acl.Manager{
		Aiven:   r.Aiven,
		Project: tx.topic.Spec.Pool,
		Service: kafkarator_aiven.ServiceName(tx.topic.Spec.Pool),
		Topic:   tx.topic,
		Logger:  tx.logger,
	}

	tx.logger.Infof("Synchronizing access control lists")
	err := aclManager.Synchronize()
	if err != nil {
		return err
	}

	userManager := serviceuser.Manager{
		AivenServiceUsers: r.Aiven.ServiceUsers,
		Project:           tx.topic.Spec.Pool,
		Service:           kafkarator_aiven.ServiceName(tx.topic.Spec.Pool),
		Logger:            tx.logger,
	}

	tx.logger.Infof("Synchronizing service users")
	users, err := userManager.Synchronize(tx.topic.Spec.ACL.Users())
	if err != nil {
		return err
	}

	tx.logger.Infof("Producing secrets")
	serviceManager := service.Manager{
		AivenCA:      r.Aiven.CA,
		AivenService: r.Aiven.Services,
		Project:      tx.topic.Spec.Pool,
		Service:      kafkarator_aiven.ServiceName(tx.topic.Spec.Pool),
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
		tx.logger = tx.logger.WithFields(log.Fields{
			"username": user.Username,
		})

		secretName, err := utils.ShortName(fmt.Sprintf("kafka-%s-%s", user.Application, tx.topic.Spec.Pool), maxSecretNameLength)
		if err != nil {
			return fmt.Errorf("unable to generate secret name: %s", err)
		}

		key := client.ObjectKey{
			Namespace: user.Team,
			Name:      secretName,
		}

		tx.logger = tx.logger.WithFields(log.Fields{
			"secret_namespace": key.Namespace,
			"secret_name":      key.Name,
		})

		opts := secretData{
			user:     *user.AivenUser,
			name:     key.Name,
			app:      user.Application,
			pool:     tx.topic.Spec.Pool,
			team:     user.Team,
			brokers:  kafkaBrokerAddress,
			registry: kafkaSchemaRegistryAddress,
			ca:       kafkaCA,
		}
		secret := ConvertSecret(opts)

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

	tx.logger.Infof("Synchronizing topic")
	topicManager := topic.Manager{
		AivenTopics: r.Aiven.KafkaTopics,
		Project:     tx.topic.Spec.Pool,
		Service:     kafkarator_aiven.ServiceName(tx.topic.Spec.Pool),
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

func ConvertSecret(data secretData) v1.Secret {
	return v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      data.name,
			Namespace: data.team,
			Labels: map[string]string{
				"team": data.team,
			},
			Annotations: map[string]string{
				"kafka.nais.io/pool":        data.pool,
				"kafka.nais.io/application": data.app,
			},
			ResourceVersion: data.resourceVersion,
		},
		StringData: map[string]string{
			KafkaCertificate:    data.user.AccessCert,
			KafkaPrivateKey:     data.user.AccessKey,
			KafkaBrokers:        data.brokers,
			KafkaSchemaRegistry: data.registry,
			KafkaCA:             data.ca,
		},
		Type: v1.SecretTypeOpaque,
	}
}
