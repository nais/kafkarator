package topic_test

import (
	"github.com/aiven/aiven-go-client"
	"github.com/google/go-cmp/cmp/cmpopts"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"github.com/nais/kafkarator/pkg/aiven/acl/topic"
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

	existingAcls []topic.Acl
	wantedAcls   []topic.Acl
	shouldAdd    []topic.Acl
	shouldRemove []topic.Acl

	kafkaAcls []*aiven.KafkaACL
	topicAcls []kafka_nais_io_v1.TopicACL
}

func (suite *ACLFilterTestSuite) SetupSuite() {
	suite.existingAcls = []topic.Acl{
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
		{ // Delete because of old naming convention
			ID:         "abcdef",
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user2.app*",
		},
		{ // Keep
			ID:         "abcdef-new",
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user2_app_eb343e9a_*",
		},
	}

	suite.wantedAcls = []topic.Acl{
		{ // Added because existing uses wrong username
			Permission: "read",
			Topic:      FullTopic,
			Username:   "user_app_0841666a_*",
		},
		{ // Already exists
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user2_app_eb343e9a_*",
		},
		{ // Added because of new user
			Permission: "readwrite",
			Topic:      FullTopic,
			Username:   "user3_app_538859ff_*",
		},
	}

	suite.shouldAdd = []topic.Acl{
		{
			Permission: "read",
			Topic:      FullTopic,
			Username:   "user_app_0841666a_*",
		},
		{
			Permission: "readwrite",
			Topic:      FullTopic,
			Username:   "user3_app_538859ff_*",
		},
	}

	suite.shouldRemove = []topic.Acl{
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
		{
			ID:         "abcdef",
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user2.app*",
		},
	}

	suite.kafkaAcls = []*aiven.KafkaACL{
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
		{ // Delete because of old naming convention
			ID:         "abcdef",
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user2.app*",
		},
		{ // Keep
			ID:         "abcdef-new",
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user2_app_eb343e9a_*",
		},
	}

	suite.topicAcls = []kafka_nais_io_v1.TopicACL{
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
}

func (suite *ACLFilterTestSuite) TestNewACLs() {
	added := topic.NewACLs(suite.existingAcls, suite.wantedAcls)

	assert.DeepEqual(suite.T(), suite.shouldAdd, added, cmpopts.SortSlices(func(a, b topic.Acl) bool {
		return a.Username < b.Username
	}))
}

func (suite *ACLFilterTestSuite) TestDeleteACLs() {
	removed := topic.DeleteACLs(suite.existingAcls, suite.wantedAcls)

	assert.DeepEqual(suite.T(), suite.shouldRemove, removed, cmpopts.SortSlices(func(a, b *topic.Acl) bool {
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
			ACL:  suite.topicAcls,
		},
	}

	m := &topic.MockInterface{}
	m.On("List", TestPool, TestService).
		Once().
		Return(suite.kafkaAcls, nil)
	m.On("Create", TestPool, TestService, mock.Anything).
		Times(2).
		Return(nil, nil)
	m.On("Delete", TestPool, TestService, mock.Anything).
		Times(4).
		Return(nil)

	aclManager := topic.Manager{
		AivenACLs: m,
		Project:   TestPool,
		Service:   TestService,
		Source:    topic.TopicAdapter{Topic: &source},
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

	m := &topic.MockInterface{}
	m.On("List", TestPool, TestService).
		Once().
		Return(suite.kafkaAcls, nil)
	m.On("Create", TestPool, TestService, mock.Anything).
		Times(1).
		Return(nil, nil)

	aclManager := topic.Manager{
		AivenACLs: m,
		Project:   TestPool,
		Service:   TestService,
		Source:    topic.StreamAdapter{Stream: &source},
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
