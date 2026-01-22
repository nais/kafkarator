package acl_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
)

const (
	TestPool    = "some-pool"
	TestService = "kafka"
	Team        = "test"
	Topic       = "topic"
	FullTopic   = "test.topic"
)

type ACLFilterTestSuite struct {
	suite.Suite

	existingAcls acl.ExistingAcls
	wantedAcls   acl.Acls

	shouldAdd    acl.Acls
	shouldRemove acl.ExistingAcls

	existingFromAiven acl.ExistingAcls
	topicAcls         []kafka_nais_io_v1.TopicACL
}

func (suite *ACLFilterTestSuite) SetupSuite() {
	suite.existingAcls = acl.ExistingAcls{
		{ // Delete because of username
			Acl: acl.Acl{
				Permission:      "read",
				Topic:           FullTopic,
				Username:        "user.app-f1fbd6bd",
				ResourcePattern: "LITERAL",
			},
			IDs: []string{"abc"},
		},
		{ // Delete because of wrong permission
			Acl: acl.Acl{
				Permission:      "write",
				Topic:           FullTopic,
				Username:        "user.app*",
				ResourcePattern: "LITERAL",
			},
			IDs: []string{"abcde"},
		},
		{ // Delete because of username and permission
			Acl: acl.Acl{
				Permission:      "read",
				Topic:           FullTopic,
				Username:        "user2.app-4ca551f9",
				ResourcePattern: "LITERAL",
			},
			IDs: []string{"123"},
		},
		{ // Delete because of old naming convention
			Acl: acl.Acl{
				Permission:      "write",
				Topic:           FullTopic,
				Username:        "user2.app*",
				ResourcePattern: "LITERAL",
			},
			IDs: []string{"abcdef"},
		},
		{ // Keep
			Acl: acl.Acl{
				Permission:      "write",
				Topic:           FullTopic,
				Username:        "user2_app_eb343e9a_*",
				ResourcePattern: "LITERAL",
			},
			IDs: []string{"abcdef-new"},
		},
	}

	suite.wantedAcls = acl.Acls{
		{ // Added because existing uses wrong username
			Permission:      "read",
			Topic:           FullTopic,
			Username:        "user_app_0841666a_*",
			ResourcePattern: "LITERAL",
		},
		{ // Already exists
			Permission:      "write",
			Topic:           FullTopic,
			Username:        "user2_app_eb343e9a_*",
			ResourcePattern: "LITERAL",
		},
		{ // Added because of new user
			Permission:      "readwrite",
			Topic:           FullTopic,
			Username:        "user3_app_538859ff_*",
			ResourcePattern: "LITERAL",
		},
	}

	suite.shouldAdd = acl.Acls{
		{
			Permission:      "read",
			Topic:           FullTopic,
			Username:        "user_app_0841666a_*",
			ResourcePattern: "LITERAL",
		},
		{
			Permission:      "readwrite",
			Topic:           FullTopic,
			Username:        "user3_app_538859ff_*",
			ResourcePattern: "LITERAL",
		},
	}

	suite.shouldRemove = acl.ExistingAcls{
		{
			Acl: acl.Acl{
				Permission:      "read",
				Topic:           FullTopic,
				Username:        "user.app-f1fbd6bd",
				ResourcePattern: "LITERAL",
			},
			IDs: []string{"abc"},
		},
		{
			Acl: acl.Acl{
				Permission:      "write",
				Topic:           FullTopic,
				Username:        "user.app*",
				ResourcePattern: "LITERAL",
			},
			IDs: []string{"abcde"},
		},
		{
			Acl: acl.Acl{
				Permission:      "read",
				Topic:           FullTopic,
				Username:        "user2.app-4ca551f9",
				ResourcePattern: "LITERAL",
			},
			IDs: []string{"123"},
		},
		{
			Acl: acl.Acl{
				Permission:      "write",
				Topic:           FullTopic,
				Username:        "user2.app*",
				ResourcePattern: "LITERAL",
			},
			IDs: []string{"abcdef"},
		},
	}

	suite.existingFromAiven = suite.existingAcls

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
	added := acl.NewACLs(suite.existingAcls, suite.wantedAcls)

	assert.DeepEqual(suite.T(), suite.shouldAdd, added, cmpopts.SortSlices(func(a, b acl.Acl) bool {
		return a.Username < b.Username
	}))
}

func (suite *ACLFilterTestSuite) TestDeleteACLs() {
	removed := acl.DeleteACLs(suite.existingAcls, suite.wantedAcls)

	assert.DeepEqual(suite.T(), suite.shouldRemove, removed, cmpopts.SortSlices(func(a, b acl.ExistingAcl) bool {
		aid := ""
		if len(a.IDs) > 0 {
			aid = a.IDs[0]
		}
		bid := ""
		if len(b.IDs) > 0 {
			bid = b.IDs[0]
		}
		return aid < bid
	}),
	)
}

func (suite *ACLFilterTestSuite) TestSynchronizeTopic() {
	ctx := context.Background()
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

	m := &acl.MockInterface{}
	m.On("List", ctx, TestPool, TestService).
		Once().
		Return(suite.existingFromAiven, nil)
	m.On("Create", ctx, TestPool, TestService, mock.Anything).
		Times(2).
		Return([]*acl.Acl{}, nil)
	m.On("Delete", ctx, TestPool, TestService, mock.Anything).
		Times(4).
		Return(nil)

	aclManager := acl.Manager{
		AivenACLs: m,
		Project:   TestPool,
		Service:   TestService,
		Source:    acl.TopicAdapter{Topic: &source},
		Logger:    log.New(),
	}

	err := aclManager.Synchronize(ctx)
	suite.NoError(err)

	m.AssertExpectations(suite.T())
}

func (suite *ACLFilterTestSuite) TestSynchronizeStream() {
	ctx := context.Background()
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
	m.On("List", ctx, TestPool, TestService).
		Once().
		Return(suite.existingFromAiven, nil)
	m.On("Create", ctx, TestPool, TestService, mock.Anything).
		Times(1).
		Return([]*acl.Acl{}, nil)

	aclManager := acl.Manager{
		AivenACLs: m,
		Project:   TestPool,
		Service:   TestService,
		Source:    acl.StreamAdapter{Stream: &source},
		Logger:    log.New(),
	}

	err := aclManager.Synchronize(ctx)
	suite.NoError(err)

	m.AssertExpectations(suite.T())
}

func TestACLFilter(t *testing.T) {
	testSuite := new(ACLFilterTestSuite)
	suite.Run(t, testSuite)
}
