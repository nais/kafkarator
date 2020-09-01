package kafka_nais_io_v1_test

import (
	"sort"
	"testing"

	"github.com/nais/kafkarator/api/v1"
	"github.com/stretchr/testify/assert"
)

func TestTopicACLs_Usernames(t *testing.T) {
	acls := kafka_nais_io_v1.TopicACLs{
		{
			Application: "app",
			Team:        "team",
		},
		{ // duplicate
			Application: "app",
			Team:        "team",
		},
		{
			Application: "app2",
			Team:        "team",
		},
		{
			Application: "app3",
			Team:        "team2",
		},
	}

	expected := []string{
		"team__app",
		"team__app2",
		"team2__app3",
	}

	actual := acls.Usernames()

	sort.Strings(expected)
	sort.Strings(actual)

	assert.Equal(t, expected, actual)
}
