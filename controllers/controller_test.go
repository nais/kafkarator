package controllers_test

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aiven/aiven-go-client"
	"github.com/ghodss/yaml"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"github.com/nais/kafkarator/controllers"
	"github.com/nais/kafkarator/pkg/aiven"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/kafkarator/pkg/aiven/service"
	"github.com/nais/kafkarator/pkg/aiven/serviceuser"
	topic_package "github.com/nais/kafkarator/pkg/aiven/topic"
	"github.com/nais/kafkarator/pkg/certificate"
	"github.com/nais/kafkarator/pkg/certificate/mocks"
	kafkaratormetrics "github.com/nais/kafkarator/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

const (
	testDataDirectory = "testdata"

	// Use these in your test data
	wellKnownID                   = "well-known-id"
	wellKnownPassword             = "well-known-password"
	wellKnownCertificateAuthority = "well-known-certificate-authority"
	wellKnownAccessCertificate    = "well-known-access-cert"
	wellKnownAccessKey            = "well-known-access-key"
	wellKnownKeyStore             = "well-known-keystore"
	wellKnownTrustStore           = "well-known-truststore"
	wellKnownStoreSecret          = "changeme"
)

type testCase struct {
	Config testCaseConfig
	Error  *string
	Aiven  aivenSpec
	Topic  json.RawMessage
	Output controllers.ReconcileResult
}

type aivenSpec struct {
	Created  aivenCreated
	Deleted  aivenDeleted
	Existing aivenData
	Updated  aivenUpdated
}

type aivenCreated struct {
	Topics       []aiven.CreateKafkaTopicRequest
	Acls         []aiven.CreateKafkaACLRequest
	Serviceusers []aiven.CreateServiceUserRequest
}

type aivenUpdated struct {
	Topics map[string]aiven.UpdateKafkaTopicRequest
}

type aivenDeleted struct {
	Topics       []string
	Serviceusers []string
	Acls         []string
}

type aivenData struct {
	Topics       []*aiven.KafkaTopic
	Serviceusers []*aiven.ServiceUser
	Acls         []*aiven.KafkaACL
	Service      *aiven.Service
	CA           string
}

type output struct {
	Status  json.RawMessage
	Secrets []v1.Secret
}

type testCaseConfig struct {
	Description string
	Projects    []string
}

func fileReader(file string) io.Reader {
	f, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	return f
}

func aivenMockInterfaces(test testCase) kafkarator_aiven.Interfaces {
	aclMock := &acl.MockInterface{}
	caMock := &service.MockCA{}
	serviceUserMock := &serviceuser.MockInterface{}
	serviceMock := &service.MockInterface{}
	topicMock := &topic_package.MockInterface{}

	for _, project := range test.Config.Projects {
		svc := kafkarator_aiven.ServiceName(project)
		serviceMock.
			On("Get", project, svc).
			Return(test.Aiven.Existing.Service, nil)
		caMock.
			On("Get", project).
			Return(test.Aiven.Existing.CA, nil)
		aclMock.
			On("List", project, svc).
			Return(test.Aiven.Existing.Acls, nil)
		serviceUserMock.
			On("List", project, svc).
			Return(test.Aiven.Existing.Serviceusers, nil)
		topicMock.
			On("List", project, svc).
			Return(test.Aiven.Existing.Topics, nil)

		for _, topic := range test.Aiven.Existing.Topics {
			topicMock.
				On("Get", project, svc, topic.TopicName).
				Return(topic, nil)
		}

		for _, topic := range test.Aiven.Created.Topics {
			topicMock.
				On("Get", project, svc, topic.TopicName).
				Return(nil, aiven.Error{
					Status: http.StatusNotFound,
				})
			topicMock.
				On("Create", project, svc, topic).
				Return(nil)
		}

		for _, serviceUser := range test.Aiven.Created.Serviceusers {
			serviceUserMock.
				On("Create", project, svc, serviceUser).
				Return(
					&aiven.ServiceUser{
						Username:   serviceUser.Username,
						Password:   wellKnownPassword,
						Type:       "who-cares",
						AccessCert: wellKnownAccessCertificate,
						AccessKey:  wellKnownAccessKey,
					},
					nil,
				)
		}

		for _, a := range test.Aiven.Created.Acls {
			aclMock.
				On("Create", project, svc, a).
				Return(
					&aiven.KafkaACL{
						ID:         wellKnownID,
						Permission: a.Permission,
						Topic:      a.Topic,
						Username:   a.Username,
					},
					nil,
				)
		}

		for topicName, topic := range test.Aiven.Updated.Topics {
			topicMock.
				On("Update", project, svc, topicName, topic).
				Return(nil)
		}

		for _, t := range test.Aiven.Deleted.Topics {
			topicMock.
				On("Delete", project, svc, t).
				Return(nil)
		}

		for _, u := range test.Aiven.Deleted.Serviceusers {
			serviceUserMock.
				On("Delete", project, svc, u).
				Return(nil)
		}

		for _, a := range test.Aiven.Deleted.Acls {
			aclMock.
				On("Delete", project, svc, a).
				Return(nil)
		}
	}

	return kafkarator_aiven.Interfaces{
		ACLs:         aclMock,
		CA:           caMock,
		ServiceUsers: serviceUserMock,
		Service:      serviceMock,
		Topics:       topicMock,
	}
}

func yamlSubTest(t *testing.T, path string) {
	fixture := fileReader(path)
	data, err := ioutil.ReadAll(fixture)
	if err != nil {
		t.Errorf("unable to read test data: %s", err)
		t.Fail()
		return
	}

	test := testCase{}
	err = yaml.Unmarshal(data, &test)
	if err != nil {
		t.Errorf("unable to unmarshal test data: %s", err)
		t.Fail()
		return
	}

	topic := &kafka_nais_io_v1.Topic{}
	err = json.Unmarshal(test.Topic, topic)
	if err != nil {
		t.Errorf("unable to parse topic: %s", err)
		t.Fail()
		return
	}

	generatorMock := &mocks.Generator{}
	aivenMocks := aivenMockInterfaces(test)

	generatorMock.
		On("MakeCredStores", wellKnownAccessKey, wellKnownAccessCertificate, wellKnownCertificateAuthority).
		Return(&certificate.CredStoreData{
			Keystore:   []byte(wellKnownKeyStore),
			Truststore: []byte(wellKnownTrustStore),
			Secret:     wellKnownStoreSecret,
		}, nil)

	reconciler := controllers.TopicReconciler{
		Aiven:               aivenMocks,
		Logger:              log.New(),
		Projects:            test.Config.Projects,
		CredentialsLifetime: 3600,
		StoreGenerator:      generatorMock,
	}

	result := reconciler.Process(*topic, log.NewEntry(log.StandardLogger()))
	if test.Error != nil {
		assert.EqualError(t, result.Error, *test.Error)
		return
	}

	// hard to test current time with static data
	test.Output.Status.CredentialsExpiryTime = result.Status.CredentialsExpiryTime
	test.Output.Status.SynchronizationTime = result.Status.SynchronizationTime
	test.Output.Status.SynchronizationHash = result.Status.SynchronizationHash

	assert.Equal(t, test.Output.Status, result.Status)
	assert.Equal(t, test.Output.Requeue, result.Requeue)
	assert.ElementsMatch(t, test.Output.Secrets, result.Secrets)
}

func TestGoldenFile(t *testing.T) {
	kafkaratormetrics.Register(prometheus.DefaultRegisterer)

	files, err := ioutil.ReadDir(testDataDirectory)
	if err != nil {
		t.Error(err)
		t.Fail()
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		path := filepath.Join(testDataDirectory, name)
		t.Run(name, func(t *testing.T) {
			yamlSubTest(t, path)
		})
	}
}

func TestGenerateCredentialRotationTime(t *testing.T) {
	lifetime := time.Hour * 1
	minHour := 1
	maxHour := 2
	increase := time.Hour * 23
	tm := controllers.GenerateCredentialRotationTime(lifetime, minHour, maxHour, increase)
	diff := tm.Sub(time.Now())

	assert.GreaterOrEqual(t, diff.Seconds(), lifetime.Seconds())
	assert.LessOrEqual(t, tm.Hour(), maxHour)
	assert.GreaterOrEqual(t, tm.Hour(), minHour)
}
