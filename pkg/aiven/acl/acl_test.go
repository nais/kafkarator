package acl_test

import (
	"github.com/google/go-cmp/cmp/cmpopts"
	kafkarator_aiven "github.com/nais/kafkarator/pkg/aiven"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
)

const (
	TestPool    = "nav-integration-test"
	TestService = "nav-integration-test-kafka"
	Team        = "test"
	Topic       = "topic"
	FullTopic   = "test.topic"
)

type ACLFilterTestSuite struct {
	suite.Suite

	existingACLs []*aiven.KafkaACL
	aclSpecs     []kafka_nais_io_v1.TopicACL
	shouldAdd    []kafka_nais_io_v1.TopicACL
	shouldRemove []*aiven.KafkaACL
}

func (suite *ACLFilterTestSuite) SetupSuite() {
	suite.existingACLs = []*aiven.KafkaACL{
		{ // Delete because of username
			ID:         "abc",
			Permission: "read",
			Topic:      FullTopic,
			Username:   "user.app-f1fbd6bd",
		},
		{ // Delete because of wrong permission
			ID:         "abcde",
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user.app*",
		},
		{ // Delete because of username and permission
			ID:         "123",
			Permission: "read",
			Topic:      FullTopic,
			Username:   "user2.app-4ca551f9",
		},
		{ // Keep
			ID:         "abcdef",
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user2.app*",
		},
	}

	suite.aclSpecs = []kafka_nais_io_v1.TopicACL{
		{ // Added because existing uses wrong username
			Access:      "read",
			Team:        "user",
			Application: "app",
		},
		{ // Already exists
			Access:      "write",
			Team:        "user2",
			Application: "app",
		},
		{ // Added because of new user
			Access:      "readwrite",
			Team:        "user3",
			Application: "app",
		},
	}

	suite.shouldAdd = []kafka_nais_io_v1.TopicACL{
		{
			Access:      "read",
			Team:        "user",
			Application: "app",
		},
		{
			Access:      "readwrite",
			Team:        "user3",
			Application: "app",
		},
	}

	suite.shouldRemove = []*aiven.KafkaACL{
		{
			ID:         "abc",
			Permission: "read",
			Topic:      FullTopic,
			Username:   "user.app-f1fbd6bd",
		},
		{
			ID:         "abcde",
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user.app*",
		},
		{
			ID:         "123",
			Permission: "read",
			Topic:      FullTopic,
			Username:   "user2.app-4ca551f9",
		},
	}
}

func (suite *ACLFilterTestSuite) TestNewACLs() {
	added := acl.NewACLs(suite.existingACLs, suite.aclSpecs)

	assert.DeepEqual(suite.T(), suite.shouldAdd, added, cmpopts.SortSlices(func(a, b kafka_nais_io_v1.TopicACL) bool {
		return a.Application < b.Application
	}))
}

func (suite *ACLFilterTestSuite) TestDeleteACLs() {
	removed := acl.DeleteACLs(suite.existingACLs, suite.aclSpecs)

	assert.DeepEqual(suite.T(), suite.shouldRemove, removed, cmpopts.SortSlices(func(a, b *aiven.KafkaACL) bool {
		return a.ID < b.ID
	}))
}

func (suite *ACLFilterTestSuite) TestSynchronizeTopic() {
	source := kafka_nais_io_v1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Topic,
			Namespace: Team,
		},
		Spec: kafka_nais_io_v1.TopicSpec{
			Pool: TestPool,
			ACL:  suite.aclSpecs,
		},
	}

	m := &acl.MockInterface{}
	m.On("List", TestPool, TestService).
		Once().
		Return(suite.existingACLs, nil)
	m.On("Create", TestPool, TestService, mock.Anything).
		Times(2).
		Return(nil, nil)
	m.On("Delete", TestPool, TestService, mock.Anything).
		Times(3).
		Return(nil)

	aclManager := acl.Manager{
		AivenACLs: m,
		Project:   TestPool,
		Service:   kafkarator_aiven.ServiceName(TestPool),
		Source:    acl.TopicAdapter{Topic: &source},
		Logger:    log.New(),
	}

	err := aclManager.Synchronize()
	suite.NoError(err)

	m.AssertExpectations(suite.T())
}

func (suite *ACLFilterTestSuite) TestSynchronizeStream() {
	source := kafka_nais_io_v1.Stream{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Topic,
			Namespace: Team,
		},
		Spec: kafka_nais_io_v1.StreamSpec{
			Pool: TestPool,
		},
	}

	m := &acl.MockInterface{}
	m.On("List", TestPool, TestService).
		Once().
		Return(suite.existingACLs, nil)
	m.On("Create", TestPool, TestService, mock.Anything).
		Times(1).
		Return(nil, nil)

	aclManager := acl.Manager{
		AivenACLs: m,
		Project:   TestPool,
		Service:   kafkarator_aiven.ServiceName(TestPool),
		Source:    acl.StreamAdapter{Stream: &source},
		Logger:    log.New(),
	}

	err := aclManager.Synchronize()
	suite.NoError(err)

	m.AssertExpectations(suite.T())
}

func TestACLFilter(t *testing.T) {
	testSuite := new(ACLFilterTestSuite)
	suite.Run(t, testSuite)
}
