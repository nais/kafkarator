package controllers_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nais/kafkarator/pkg/utils"
	"github.com/nais/liberator/pkg/aiven/service"
	"github.com/stretchr/testify/mock"

	"github.com/aiven/aiven-go-client/v2"
	"github.com/ghodss/yaml"
	"github.com/nais/kafkarator/controllers"
	"github.com/nais/kafkarator/pkg/aiven"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	topic_package "github.com/nais/kafkarator/pkg/aiven/topic"
	kafkaratormetrics "github.com/nais/kafkarator/pkg/metrics"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"gotest.tools/assert"
)

const (
	testDataDirectory = "testdata"

	// Use these in your test data
	wellKnownID = "well-known-id"
)

type testCase struct {
	Config testCaseConfig
	Error  *string
	Aiven  aivenSpec
	Topic  json.RawMessage
	Output controllers.TopicReconcileResult
}

type aivenSpec struct {
	Created  aivenCreated
	Deleted  aivenDeleted
	Existing aivenData
	Updated  aivenUpdated
	Missing  aivenMissing
}

type aivenCreated struct {
	Topics []aiven.CreateKafkaTopicRequest
	Acls   []acl.CreateKafkaACLRequest
}

type aivenUpdated struct {
	Topics map[string]aiven.UpdateKafkaTopicRequest
}

type aivenDeleted struct {
	Topics []string
	Acls   []string
}

type aivenMissing struct {
	Topics []string
}

type aivenData struct {
	Topics []*aiven.KafkaTopic
	Acls   []*acl.Acl
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

func aivenMockInterfaces(ctx context.Context, t *testing.T, test testCase) (kafkarator_aiven.Interfaces, func(t mock.TestingT) bool) {
	notFoundError := aiven.Error{
		Message:  "Not Found",
		MoreInfo: "",
		Status:   404,
	}

	mockNameResolver := service.NewMockNameResolver(t)
	mockNameResolver.On("ResolveKafkaServiceName", ctx, mock.Anything).Return("kafka", nil)

	aclMock := &acl.MockInterface{}
	aclMock.Test(t)
	topicMock := &topic_package.MockInterface{}
	topicMock.Test(t)

	for _, project := range test.Config.Projects {
		svc, _ := mockNameResolver.ResolveKafkaServiceName(ctx, project)
		aclMock.
			On("List", ctx, project, svc).
			Maybe().
			Return(test.Aiven.Existing.Acls, nil)

		for _, topic := range test.Aiven.Missing.Topics {
			topicMock.
				On("Get", ctx, project, svc, topic).
				Maybe().
				Return(nil, notFoundError)
			topicMock.
				On("Delete", ctx, project, svc, topic).
				Maybe().
				Return(notFoundError)
		}

		for _, topic := range test.Aiven.Existing.Topics {
			topicMock.
				On("Get", ctx, project, svc, topic.TopicName).
				Maybe().
				Return(topic, nil)
		}

		for _, topic := range test.Aiven.Created.Topics {
			topicMock.
				On("Get", ctx, project, svc, topic.TopicName).
				Maybe().
				Return(nil, aiven.Error{
					Status: http.StatusNotFound,
				})
			topicMock.
				On("Create", ctx, project, svc, mock.MatchedBy(utils.TopicCreateReqComp(topic))).
				Return(nil)
		}

		for _, a := range test.Aiven.Created.Acls {
			aclMock.
				On("Create", ctx, project, svc, a).
				Return(
					&acl.Acl{
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
				On("Update", ctx, project, svc, topicName, mock.MatchedBy(utils.TopicUpdateReqComp(topic))).
				Return(nil)
		}

		for _, topicName := range test.Aiven.Deleted.Topics {
			topicMock.
				On("Delete", ctx, project, svc, topicName).
				Return(nil)
		}

		for _, a := range test.Aiven.Deleted.Acls {
			aclMock.
				On("Delete", ctx, project, svc, a).
				Return(nil)
		}
	}

	return kafkarator_aiven.Interfaces{
			ACLs:         aclMock,
			Topics:       topicMock,
			NameResolver: mockNameResolver,
		}, func(t mock.TestingT) bool {
			result := false
			if ok := aclMock.AssertExpectations(t); !ok {
				t.Errorf("Expectations on ACLs failed")
				result = result || ok
			}
			if ok := topicMock.AssertExpectations(t); !ok {
				t.Errorf("Expectations on Topic failed")
				result = result || ok
			}
			return result
		}
}

func yamlSubTest(ctx context.Context, t *testing.T, path string) {
	fixture := fileReader(path)
	data, err := io.ReadAll(fixture)
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

	aivenMocks, assertMocks := aivenMockInterfaces(ctx, t, test)

	reconciler := controllers.TopicReconciler{
		Aiven:    aivenMocks,
		Logger:   log.New(),
		Projects: test.Config.Projects,
	}

	result := reconciler.Process(ctx, *topic, log.NewEntry(log.StandardLogger()))
	if test.Error != nil {
		assert.Equal(t, result.Error, *test.Error)
		return
	}

	// hard to test current time with static data
	test.Output.Status.SynchronizationTime = result.Status.SynchronizationTime
	test.Output.Status.SynchronizationHash = result.Status.SynchronizationHash

	assert.DeepEqual(t, test.Output.Status, result.Status)
	assert.Equal(t, test.Output.Requeue, result.Requeue)
	assertMocks(t)
}

func TestGoldenFile(t *testing.T) {
	kafkaratormetrics.Register(prometheus.DefaultRegisterer)
	ctx := context.Background()

	files, err := os.ReadDir(testDataDirectory)
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
			yamlSubTest(ctx, t, path)
		})
	}
}
