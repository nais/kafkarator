package acl_test

import (
	"testing"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/api/v1"
	"github.com/nais/kafkarator/pkg/aiven/acl"
	"github.com/stretchr/testify/assert"
)

func TestACLFilter(t *testing.T) {
	existingACLs := []*aiven.KafkaACL{
		{
			ID:         "abc",
			Permission: "read",
			Topic:      "topic",
			Username:   "user__app-fe8425fc",
		},
		{
			ID:         "abcde",
			Permission: "write",
			Topic:      "topic",
			Username:   "user__app-fe8425fc",
		},
		{
			ID:         "123",
			Permission: "read",
			Topic:      "topic",
			Username:   "user2__app-8f88fef2",
		},
		{
			ID:         "abcdef",
			Permission: "write",
			Topic:      "topic",
			Username:   "user2__app-8f88fef2",
		},
		{
			ID:         "abc",
			Permission: "read",
			Topic:      "not_our_topic",
			Username:   "user__app-fe8425fc",
		},
	}

	aclSpecs := []kafka_nais_io_v1.TopicACL{
		{
			Access:      "read",
			Team:        "user",
			Application: "app",
		},
		{
			Access:      "write",
			Team:        "user2",
			Application: "app",
		},
		{
			Access:      "readwrite",
			Team:        "user3",
			Application: "app",
		},
	}

	shouldAdd := []kafka_nais_io_v1.TopicACL{
		{
			Access:      "readwrite",
			Team:        "user3",
			Application: "app",
		},
	}

	shouldRemove := []*aiven.KafkaACL{
		{
			ID:         "abcde",
			Permission: "write",
			Topic:      "topic",
			Username:   "user__app-fe8425fc",
		},
		{
			ID:         "123",
			Permission: "read",
			Topic:      "topic",
			Username:   "user2__app-8f88fef2",
		},
	}

	added := acl.NewACLs(existingACLs, aclSpecs)
	removed := acl.DeleteACLs(existingACLs, aclSpecs)

	assert.Equal(t, shouldAdd, added)

	assert.Equal(t, shouldRemove, removed)
}
