package kafka_nais_io_v1_test

import (
	"sort"
	"testing"

	"github.com/nais/kafkarator/api/v1"
	"github.com/stretchr/testify/assert"
)

type UserList []kafka_nais_io_v1.User

func (ul UserList) Len() int {
	return len(ul)
}

func (ul UserList) Less(i, j int) bool {
	return ul[i].Username < ul[j].Username
}

func (ul UserList) Swap(i, j int) {
	x := ul[i]
	ul[i] = ul[j]
	ul[j] = x
}

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

	expected := UserList{
		{
			Username:    "team__app-55e49d4e",
			Application: "app",
			Team:        "team",
		},
		{
			Username:    "team__app2-8be43607",
			Application: "app2",
			Team:        "team",
		},
		{
			Username:    "team2__app3-2da7c9b1",
			Application: "app3",
			Team:        "team2",
		},
	}

	actual := UserList(acls.Users())

	sort.Sort(expected)
	sort.Sort(actual)

	assert.Equal(t, expected, actual)
}
