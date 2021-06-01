package acl_test

import (
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/suite"
	"gotest.tools/assert"
	"testing"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
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
			Topic:      "topic",
			Username:   "user.app-f1fbd6bd",
		},
		{ // Delete because of wrong permission
			ID:         "abcde",
			Permission: "write",
			Topic:      "topic",
			Username:   "user.app*",
		},
		{ // Delete because of username and permission
			ID:         "123",
			Permission: "read",
			Topic:      "topic",
			Username:   "user2.app-4ca551f9",
		},
		{ // Keep
			ID:         "abcdef",
			Permission: "write",
			Topic:      "topic",
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
			Topic:      "topic",
			Username:   "user.app-f1fbd6bd",
		},
		{
			ID:         "abcde",
			Permission: "write",
			Topic:      "topic",
			Username:   "user.app*",
		},
		{
			ID:         "123",
			Permission: "read",
			Topic:      "topic",
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

func TestACLFilter(t *testing.T) {
	testSuite := new(ACLFilterTestSuite)
	suite.Run(t, testSuite)
}
