package controllers

import (
	"fmt"

	"github.com/nais/kafkarator/api/v1"
	"github.com/nais/kafkarator/pkg/aiven"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/kafkarator/pkg/aiven/service"
	"github.com/nais/kafkarator/pkg/aiven/serviceuser"
	"github.com/nais/kafkarator/pkg/aiven/topic"
	"github.com/nais/kafkarator/pkg/certificate"
	"github.com/nais/kafkarator/pkg/utils"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Synchronizer struct {
	ACLs           acl.Manager
	Topics         topic.Manager
	Users          serviceuser.Manager
	Services       service.Manager
	Logger         *log.Entry
	StoreGenerator certificate.Generator
}

type SyncResult struct {
	brokers  string
	registry string
	ca       string
	users    []*serviceuser.UserMap
	topic    kafka_nais_io_v1.Topic
}

func NewSynchronizer(a kafkarator_aiven.Interfaces, t kafka_nais_io_v1.Topic, logger *log.Entry) *Synchronizer {
	return &Synchronizer{
		Logger: logger,
		Topics: topic.Manager{
			AivenTopics: a.Topics,
			Project:     t.Spec.Pool,
			Service:     kafkarator_aiven.ServiceName(t.Spec.Pool),
			Topic:       t,
			Logger:      logger,
		},
		ACLs: acl.Manager{
			AivenACLs: a.ACLs,
			Project:   t.Spec.Pool,
			Service:   kafkarator_aiven.ServiceName(t.Spec.Pool),
			Topic:     t,
			Logger:    logger,
		},
		Users: serviceuser.Manager{
			AivenServiceUsers: a.ServiceUsers,
			Project:           t.Spec.Pool,
			Service:           kafkarator_aiven.ServiceName(t.Spec.Pool),
			Logger:            logger,
		},
		Services: service.Manager{
			AivenCA:      a.CA,
			AivenService: a.Service,
			Project:      t.Spec.Pool,
			Service:      kafkarator_aiven.ServiceName(t.Spec.Pool),
			Logger:       logger,
		},
	}
}

func (c *Synchronizer) Synchronize(topic kafka_nais_io_v1.Topic) (*SyncResult, error) {
	c.Logger.Infof("Retrieving service information")
	svc, err := c.Services.Get()
	if err != nil {
		return nil, err
	}

	kafkaBrokerAddress := service.GetKafkaBrokerAddress(*svc)
	kafkaSchemaRegistryAddress := service.GetSchemaRegistryAddress(*svc)

	kafkaCA, err := c.Services.GetCA()
	if err != nil {
		return nil, err
	}

	c.Logger.Infof("Synchronizing access control lists")
	err = c.ACLs.Synchronize()
	if err != nil {
		return nil, err
	}

	c.Logger.Infof("Synchronizing service users")
	users, err := c.Users.Synchronize(topic.Spec.ACL.Users())
	if err != nil {
		return nil, err
	}

	c.Logger.Infof("Synchronizing topic")
	err = c.Topics.Synchronize()
	if err != nil {
		return nil, err
	}

	return &SyncResult{
		brokers:  kafkaBrokerAddress,
		registry: kafkaSchemaRegistryAddress,
		ca:       kafkaCA,
		users:    users,
		topic:    topic,
	}, nil
}

func Secret(topic kafka_nais_io_v1.Topic, generator certificate.Generator, user serviceuser.UserMap, brokers, registry, ca string) (*v1.Secret, error) {
	secretName, err := utils.ShortName(fmt.Sprintf("kafka-%s-%s", user.Application, topic.Spec.Pool), maxSecretNameLength)
	if err != nil {
		return nil, fmt.Errorf("unable to generate secret name: %s", err)
	}

	credStore, err := generator.MakeCredStores(user.AivenUser.AccessKey, user.AivenUser.AccessCert, ca)
	if err != nil {
		return nil, fmt.Errorf("unable to generate truststore/keystore: %w", err)
	}

	key := client.ObjectKey{
		Namespace: user.Team,
		Name:      secretName,
	}

	return &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels: map[string]string{
				"team": user.Team,
			},
			Annotations: map[string]string{
				"kafka.nais.io/pool":        topic.Spec.Pool,
				"kafka.nais.io/application": user.Application,
			},
		},
		StringData: map[string]string{
			KafkaCertificate:       user.AivenUser.AccessCert,
			KafkaPrivateKey:        user.AivenUser.AccessKey,
			KafkaBrokers:           brokers,
			KafkaSchemaRegistry:    registry,
			KafkaCA:                ca,
			KafkaCredStorePassword: credStore.Secret,
		},
		Data: map[string][]byte{
			KafkaKeystore:   credStore.Keystore,
			KafkaTruststore: credStore.Truststore,
		},
		Type: v1.SecretTypeOpaque,
	}, nil
}
