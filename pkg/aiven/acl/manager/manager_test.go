package manager_test

import (
	"testing"

	"github.com/aiven/aiven-go-client"
	"github.com/google/go-cmp/cmp/cmpopts"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/nais/kafkarator/pkg/aiven/acl/manager"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
)

const (
	TestPool    = "nav-integration-test"
	TestService = "nav-integration-test-kafka"
	Team        = "test"
	Topic       = "topic"
	FullTopic   = "test.topic"
	Resource    = "Subject:test.topic"
)

type ACLFilterTestSuite struct {
	suite.Suite

	existingAcls []manager.Acl
	wantedAcls   []manager.Acl
	shouldAdd    []manager.Acl
	shouldRemove []manager.Acl

	kafkaAcls []*aiven.KafkaSchemaRegistryACL
	topicAcls []kafka_nais_io_v1.TopicACL
}

func (suite *ACLFilterTestSuite) SetupSuite() {
	suite.existingAcls = []manager.Acl{
		{ // Delete because of username
			ID:           "abc",
			Permission:   "read",
			TopicPattern: FullTopic,
			Username:     "user.app-f1fbd6bd",
		},
		{ // Delete because of wrong permission
			ID:           "abcde",
			Permission:   "write",
			TopicPattern: FullTopic,
			Username:     "user.app*",
		},
		{ // Delete because of username and permission
			ID:           "123",
			Permission:   "read",
			TopicPattern: FullTopic,
			Username:     "user2.app-4ca551f9",
		},
		{ // Delete because of old naming convention
			ID:           "abcdef",
			Permission:   "write",
			TopicPattern: FullTopic,
			Username:     "user2.app*",
		},
		{ // Keep
			ID:           "abcdef-new",
			Permission:   "write",
			TopicPattern: FullTopic,
			Username:     "user2_app_eb343e9a_*",
		},
	}

	suite.wantedAcls = []manager.Acl{
		{ // Added because existing uses wrong username
			Permission:   "read",
			TopicPattern: FullTopic,
			Username:     "user_app_0841666a_*",
		},
		{ // Already exists
			Permission:   "write",
			TopicPattern: FullTopic,
			Username:     "user2_app_eb343e9a_*",
		},
		{ // Added because of new user
			Permission:   "readwrite",
			TopicPattern: FullTopic,
			Username:     "user3_app_538859ff_*",
		},
	}

	suite.shouldAdd = []manager.Acl{
		{
			Permission:   "read",
			TopicPattern: FullTopic,
			Username:     "user_app_0841666a_*",
		},
		{
			Permission:   "readwrite",
			TopicPattern: FullTopic,
			Username:     "user3_app_538859ff_*",
		},
	}

	suite.shouldRemove = []manager.Acl{
		{
			ID:           "abc",
			Permission:   "read",
			TopicPattern: FullTopic,
			Username:     "user.app-f1fbd6bd",
		},
		{
			ID:           "abcde",
			Permission:   "write",
			TopicPattern: FullTopic,
			Username:     "user.app*",
		},
		{
			ID:           "123",
			Permission:   "read",
			TopicPattern: FullTopic,
			Username:     "user2.app-4ca551f9",
		},
		{
			ID:           "abcdef",
			Permission:   "write",
			TopicPattern: FullTopic,
			Username:     "user2.app*",
		},
	}

	suite.kafkaAcls = []*aiven.KafkaSchemaRegistryACL{
		{ // Delete because of username
			ID:         "abc",
			Permission: "read",
			Resource:   Resource,
			Username:   "user.app-f1fbd6bd",
		},
		{ // Delete because of wrong permission
			ID:         "abcde",
			Permission: "write",
			Resource:   Resource,
			Username:   "user.app*",
		},
		{ // Delete because of username and permission
			ID:         "123",
			Permission: "read",
			Resource:   Resource,
			Username:   "user2.app-4ca551f9",
		},
		{ // Delete because of old naming convention
			ID:         "abcdef",
			Permission: "write",
			Resource:   Resource,
			Username:   "user2.app*",
		},
		{ // Keep
			ID:         "abcdef-new",
			Permission: "write",
			Resource:   Resource,
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
	added := manager.NewACLs(suite.existingAcls, suite.wantedAcls)

	assert.DeepEqual(suite.T(), suite.shouldAdd, added, cmpopts.SortSlices(func(a, b manager.Acl) bool {
		return a.Username < b.Username
	}))
}

func (suite *ACLFilterTestSuite) TestDeleteACLs() {
	removed := manager.DeleteACLs(suite.existingAcls, suite.wantedAcls)

	assert.DeepEqual(suite.T(), suite.shouldRemove, removed, cmpopts.SortSlices(func(a, b *manager.Acl) bool {
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

	m := manager.NewMockSchemaRegistryAclInterface(suite.T())
	m.On("List", TestPool, TestService).
		Once().
		Return(suite.kafkaAcls, nil)
	m.On("Create", TestPool, TestService, mock.Anything).
		Times(2).
		Return(&aiven.KafkaSchemaRegistryACL{}, nil)
	m.On("Delete", TestPool, TestService, mock.Anything).
		Times(4).
		Return(nil)

	aclManager := manager.Manager{
		AivenAdapter: manager.AivenSchemaRegistryACLAdapter{
			SchemaRegistryAclInterface: m,
			Project:                    TestPool,
			Service:                    TestService,
		},
		Project: TestPool,
		Service: TestService,
		Source:  manager.TopicAdapter{Topic: &source},
		Logger:  log.New(),
	}

	err := aclManager.Synchronize()
	suite.NoError(err)
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

	m := manager.NewMockSchemaRegistryAclInterface(suite.T())
	m.On("List", TestPool, TestService).
		Once().
		Return(suite.kafkaAcls, nil)
	m.On("Create", TestPool, TestService, mock.Anything).
		Times(1).
		Return(&aiven.KafkaSchemaRegistryACL{}, nil)

	aclManager := manager.Manager{
		AivenAdapter: manager.AivenSchemaRegistryACLAdapter{
			SchemaRegistryAclInterface: m,
			Project:                    TestPool,
			Service:                    TestService,
		},
		Project: TestPool,
		Service: TestService,
		Source:  manager.StreamAdapter{Stream: &source},
		Logger:  log.New(),
	}

	err := aclManager.Synchronize()
	suite.NoError(err)
}

func TestACLFilter(t *testing.T) {
	testSuite := new(ACLFilterTestSuite)
	suite.Run(t, testSuite)
}
