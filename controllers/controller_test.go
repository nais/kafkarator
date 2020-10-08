package controllers_test

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aiven/aiven-go-client"
	"github.com/ghodss/yaml"
	"github.com/nais/kafkarator/api/v1"
	"github.com/nais/kafkarator/controllers"
	"github.com/nais/kafkarator/pkg/aiven"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/kafkarator/pkg/aiven/service"
	"github.com/nais/kafkarator/pkg/aiven/serviceuser"
	topic_package "github.com/nais/kafkarator/pkg/aiven/topic"
	"github.com/nais/kafkarator/pkg/certificate/mocks"
	kafkaratormetrics "github.com/nais/kafkarator/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

const (
	testDataDirectory = "testdata"
)

type testCase struct {
	Config testCaseConfig
	Error  *string
	Aiven  aivenData
	Topic  json.RawMessage
	Output controllers.ReconcileResult
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

	aclMock := &acl.MockInterface{}
	caMock := &service.MockCA{}
	serviceUserMock := &serviceuser.MockInterface{}
	serviceMock := &service.MockInterface{}
	topicMock := &topic_package.MockInterface{}
	generatorMock := &mocks.Generator{}

	for _, project := range test.Config.Projects {
		svc := kafkarator_aiven.ServiceName(project)
		serviceMock.
			On("Get", project, svc).
			Return(test.Aiven.Service, nil)
		caMock.
			On("Get", project).
			Return(test.Aiven.CA, nil)
		aclMock.
			On("List", project, svc).
			Return(test.Aiven.Acls, nil)
		serviceUserMock.
			On("List", project, svc).
			Return(test.Aiven.Serviceusers, nil)
		topicMock.
			On("List", project, svc).
			Return(test.Aiven.Topics, nil)
	}

	aivenMocks := kafkarator_aiven.Interfaces{
		ACLs:         aclMock,
		CA:           caMock,
		ServiceUsers: serviceUserMock,
		Service:      serviceMock,
		Topics:       topicMock,
	}

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

	assert.Equal(t, test.Output, result)
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
