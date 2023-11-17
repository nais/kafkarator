package schemaregistry_test

import (
	"testing"

	"github.com/aiven/aiven-go-client"
	"github.com/google/go-cmp/cmp/cmpopts"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/nais/kafkarator/pkg/aiven/acl/schemaregistry"
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

	existingAcls []schemaregistry.Acl
	wantedAcls   []schemaregistry.Acl
	shouldAdd    []schemaregistry.Acl
	shouldRemove []schemaregistry.Acl

	kafkaSchemaAcls []*aiven.KafkaSchemaRegistryACL
	topicAcls       []kafka_nais_io_v1.TopicACL
}

func (suite *ACLFilterTestSuite) SetupSuite() {
	suite.existingAcls = []schemaregistry.Acl{
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

	suite.wantedAcls = []schemaregistry.Acl{
		{ // Added because existing uses wrong username
			Permission: "read",
			Resource:   Resource,
			Username:   "user_app_0841666a_*",
		},
		{ // Already exists
			Permission: "write",
			Resource:   Resource,
			Username:   "user2_app_eb343e9a_*",
		},
		{ // Added because of new user
			Permission: "readwrite",
			Resource:   Resource,
			Username:   "user3_app_538859ff_*",
		},
	}

	suite.shouldAdd = []schemaregistry.Acl{
		{
			Permission: "read",
			Resource:   Resource,
			Username:   "user_app_0841666a_*",
		},
		{
			Permission: "readwrite",
			Resource:   Resource,
			Username:   "user3_app_538859ff_*",
		},
	}

	suite.shouldRemove = []schemaregistry.Acl{
		{
			ID:         "abc",
			Permission: "read",
			Resource:   Resource,
			Username:   "user.app-f1fbd6bd",
		},
		{
			ID:         "abcde",
			Permission: "write",
			Resource:   Resource,
			Username:   "user.app*",
		},
		{
			ID:         "123",
			Permission: "read",
			Resource:   Resource,
			Username:   "user2.app-4ca551f9",
		},
		{
			ID:         "abcdef",
			Permission: "write",
			Resource:   Resource,
			Username:   "user2.app*",
		},
	}

	suite.kafkaSchemaAcls = []*aiven.KafkaSchemaRegistryACL{
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
	added := schemaregistry.NewACLs(suite.existingAcls, suite.wantedAcls)

	assert.DeepEqual(suite.T(), suite.shouldAdd, added, cmpopts.SortSlices(func(a, b schemaregistry.Acl) bool {
		return a.Username < b.Username
	}))
}

func (suite *ACLFilterTestSuite) TestDeleteACLs() {
	removed := schemaregistry.DeleteACLs(suite.existingAcls, suite.wantedAcls)

	assert.DeepEqual(suite.T(), suite.shouldRemove, removed, cmpopts.SortSlices(func(a, b *schemaregistry.Acl) bool {
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

	m := &schemaregistry.MockInterface{}
	m.On("List", TestPool, TestService).
		Once().
		Return(suite.kafkaSchemaAcls, nil)
	m.On("Create", TestPool, TestService, mock.Anything).
		Times(2).
		Return(nil, nil)
	m.On("Delete", TestPool, TestService, mock.Anything).
		Times(4).
		Return(nil)

	aclManager := schemaregistry.Manager{
		AivenSchemaACLs: m,
		Project:         TestPool,
		Service:         TestService,
		Source:          schemaregistry.TopicAdapter{Topic: &source},
		Logger:          log.New(),
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

	m := &schemaregistry.MockInterface{}
	m.On("List", TestPool, TestService).
		Once().
		Return(suite.kafkaSchemaAcls, nil)
	m.On("Create", TestPool, TestService, mock.Anything).
		Times(1).
		Return(nil, nil)

	aclManager := schemaregistry.Manager{
		AivenSchemaACLs: m,
		Project:         TestPool,
		Service:         TestService,
		Source:          schemaregistry.StreamAdapter{Stream: &source},
		Logger:          log.New(),
	}

	err := aclManager.Synchronize()
	suite.NoError(err)

	m.AssertExpectations(suite.T())
}

func TestACLFilter(t *testing.T) {
	testSuite := new(ACLFilterTestSuite)
	suite.Run(t, testSuite)
}
