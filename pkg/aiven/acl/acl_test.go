package acl_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	existingAcls []acl.Acl
	wantedAcls   []acl.Acl
	shouldAdd    []acl.Acl
	shouldRemove []acl.Acl

	kafkaAcls []*acl.Acl
	topicAcls []kafka_nais_io_v1.TopicACL
}

func (suite *ACLFilterTestSuite) SetupSuite() {
	suite.existingAcls = []acl.Acl{
		{ // Delete because of username
			IDs:        []string{"abc"},
			Permission: "read",
			Topic:      FullTopic,
			Username:   "user.app-f1fbd6bd",
		},
		{ // Delete because of wrong permission
			IDs:        []string{"abcde"},
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user.app*",
		},
		{ // Delete because of username and permission
			IDs:        []string{"123"},
			Permission: "read",
			Topic:      FullTopic,
			Username:   "user2.app-4ca551f9",
		},
		{ // Delete because of old naming convention
			IDs:        []string{"abcdef"},
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user2.app*",
		},
		{ // Keep
			IDs:        []string{"abcdef-new"},
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user2_app_eb343e9a_*",
		},
	}

	suite.wantedAcls = []acl.Acl{
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

	suite.shouldAdd = []acl.Acl{
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

	suite.shouldRemove = []acl.Acl{
		{
			IDs:        []string{"abc"},
			Permission: "read",
			Topic:      FullTopic,
			Username:   "user.app-f1fbd6bd",
		},
		{
			IDs:        []string{"abcde"},
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user.app*",
		},
		{
			IDs:        []string{"123"},
			Permission: "read",
			Topic:      FullTopic,
			Username:   "user2.app-4ca551f9",
		},
		{
			IDs:        []string{"abcdef"},
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user2.app*",
		},
	}

	suite.kafkaAcls = []*acl.Acl{
		{ // Delete because of username
			IDs:        []string{"abc"},
			Permission: "read",
			Topic:      FullTopic,
			Username:   "user.app-f1fbd6bd",
		},
		{ // Delete because of wrong permission
			IDs:        []string{"abcde"},
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user.app*",
		},
		{ // Delete because of username and permission
			IDs:        []string{"123"},
			Permission: "read",
			Topic:      FullTopic,
			Username:   "user2.app-4ca551f9",
		},
		{ // Delete because of old naming convention
			IDs:        []string{"abcdef"},
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user2.app*",
		},
		{ // Keep
			IDs:        []string{"abcdef-new"},
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
	added := acl.NewACLs(suite.existingAcls, suite.wantedAcls)

	assert.DeepEqual(suite.T(), suite.shouldAdd, added, cmpopts.SortSlices(func(a, b acl.Acl) bool {
		return a.Username < b.Username
	}))
}

func (suite *ACLFilterTestSuite) TestDeleteACLs() {
	removed := acl.DeleteACLs(suite.existingAcls, suite.wantedAcls)

	assert.DeepEqual(suite.T(), suite.shouldRemove, removed, cmpopts.SortSlices(func(a, b acl.Acl) bool {
		return a.Username < b.Username
	}))
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
		Return(suite.kafkaAcls, nil)
	m.On("Create", ctx, TestPool, TestService, false, mock.Anything).
		Times(2).
		Return(nil)
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
		Return(suite.kafkaAcls, nil)
	m.On("Create", ctx, TestPool, TestService, true, mock.Anything).
		Times(1).
		Return(nil)

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

func (suite *ACLFilterTestSuite) TestSynchronizeStreamWithAdditionalUsers() {
	ctx := context.Background()
	source := kafka_nais_io_v1.Stream{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Topic,
			Namespace: Team,
		},
		Spec: kafka_nais_io_v1.StreamSpec{
			Pool: TestPool,
			AdditionalUsers: []kafka_nais_io_v1.AdditionalStreamUser{
				{Username: "user1"},
				{Username: "user2"},
			},
		},
	}

	m := &acl.MockInterface{}
	m.On("List", ctx, TestPool, TestService).
		Once().
		Return(suite.kafkaAcls, nil)

	created := make([]acl.CreateKafkaACLRequest, 0)

	m.On("Create", ctx, TestPool, TestService, mock.Anything, mock.Anything).
		Times(1+len(source.Spec.AdditionalUsers)).
		Run(func(args mock.Arguments) {
			req := args.Get(4).(acl.CreateKafkaACLRequest)
			created = append(created, req)
		}).
		Return(nil, nil)

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

	for _, req := range created {
		assert.Equal(suite.T(), "admin", req.Permission)
		assert.Equal(suite.T(), source.TopicWildcard(), req.Topic)
	}

	wantPrefixes := map[string]bool{
		Team + "_" + Topic + "_": false,
		Team + "_user1_":         false,
		Team + "_user2_":         false,
	}

	for _, req := range created {
		for p := range wantPrefixes {
			if strings.HasPrefix(req.Username, p) {
				wantPrefixes[p] = true
			}
		}
	}

	for p, seen := range wantPrefixes {
		assert.Assert(suite.T(), seen, "expected a created username with prefix %q, but didn't see it; got=%v", p, created)
	}
}

func (suite *ACLFilterTestSuite) TestSynchronizeTopic_DeletesAllNativeIDs() {
	ctx := context.Background()

	// read => describe + read
	// write => describe + write
	// readwrite => describe + read + write
	kafkaAcls := []*acl.Acl{
		{
			IDs:        []string{"u1-describe", "u1-read"},
			Permission: "read",
			Topic:      FullTopic,
			Username:   "user.app-f1fbd6bd",
		},
		{
			IDs:        []string{"u2-describe", "u2-write"},
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user.app*",
		},
		{
			IDs:        []string{"u3-describe", "u3-read", "u3-write"},
			Permission: "readwrite",
			Topic:      FullTopic,
			Username:   "user2.app-4ca551f9",
		},
		{
			IDs:        []string{"keep-describe", "keep-write"},
			Permission: "write",
			Topic:      FullTopic,
			Username:   "user2_app_eb343e9a_*", // kept
		},
	}

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
		Return(kafkaAcls, nil)

	seen := collectDeletedIDs(ctx, m)

	m.On("Create", ctx, TestPool, TestService, false, acl.CreateKafkaACLRequest{
		Permission: "read",
		Topic:      FullTopic,
		Username:   "user_app_0841666a_*",
	}).Once().Return(nil)

	m.On("Create", ctx, TestPool, TestService, false, acl.CreateKafkaACLRequest{
		Permission: "readwrite",
		Topic:      FullTopic,
		Username:   "user3_app_538859ff_*",
	}).Once().Return(nil)

	aclManager := acl.Manager{
		AivenACLs: m,
		Project:   TestPool,
		Service:   TestService,
		Source:    acl.TopicAdapter{Topic: &source},
		Logger:    log.New(),
	}

	err := aclManager.Synchronize(ctx)
	suite.NoError(err)

	expectedDelete := []string{
		// user.app-f1fbd6bd (read)
		"u1-describe", "u1-read",

		// user.app* (write)
		"u2-describe", "u2-write",

		// user2.app-4ca551f9 (readwrite)
		"u3-describe", "u3-read", "u3-write",
	}

	for _, id := range expectedDelete {
		if seen[id] == 0 {
			suite.T().Errorf("expected native id to be deleted but wasn't seen: %s", id)
		}
	}

	if seen["keep-describe"] > 0 || seen["keep-write"] > 0 {
		suite.T().Errorf("saw deletion of kept ACL ids: %#v", seen)
	}

	m.AssertExpectations(suite.T())
}

func collectDeletedIDs(ctx context.Context, m *acl.MockInterface) map[string]int {
	seen := map[string]int{}

	m.On("Delete", ctx, TestPool, TestService, mock.Anything).
		Maybe().
		Run(func(args mock.Arguments) {
			a := args.Get(3).(acl.Acl)
			for _, id := range a.IDs {
				seen[id]++
			}
		}).
		Return(nil)

	return seen
}

func TestACLFilter(t *testing.T) {
	testSuite := new(ACLFilterTestSuite)
	suite.Run(t, testSuite)
}
