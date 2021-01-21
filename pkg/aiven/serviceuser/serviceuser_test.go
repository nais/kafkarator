package serviceuser_test

import (
	"testing"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"github.com/nais/kafkarator/pkg/aiven/serviceuser"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const (
	project = "myproject"
	service = "myservice"
)

func TestManager_Synchronize(t *testing.T) {
	m := &serviceuser.MockInterface{}

	usernames := make([]kafka_nais_io_v1.User, 0)

	// Existing user, should be deleted and re-created
	usernames = append(usernames, kafka_nais_io_v1.User{
		Username:    "app1-team1",
		Application: "app1",
		Team:        "team1",
	})

	// Missing user, should be created
	usernames = append(usernames, kafka_nais_io_v1.User{
		Username:    "app2-team1",
		Application: "app2",
		Team:        "team1",
	})

	existingUsers := []*aiven.ServiceUser{
		{
			Username: "app1-team1",
		},
	}

	// Order of calls is correct
	m.
		On("List", project, service).
		Return(existingUsers, nil)
	m.
		On("Delete", project, service, "app1-team1").
		Return(nil)
	m.
		On("Create", project, service, aiven.CreateServiceUserRequest{Username: "app1-team1"}).
		Return(&aiven.ServiceUser{Username: "app1-team1"}, nil)
	m.
		On("Create", project, service, aiven.CreateServiceUserRequest{Username: "app2-team1"}).
		Return(&aiven.ServiceUser{Username: "app2-team1"}, nil)

	manager := serviceuser.Manager{
		AivenServiceUsers: m,
		Project:           project,
		Service:           service,
		Logger:            log.NewEntry(log.StandardLogger()),
	}

	expectedReturnUsers := []*serviceuser.UserMap{
		{
			User: usernames[0],
			AivenUser: &aiven.ServiceUser{
				Username: "app1-team1",
			},
		},
		{
			User: usernames[1],
			AivenUser: &aiven.ServiceUser{
				Username: "app2-team1",
			},
		},
	}
	users, err := manager.Synchronize(usernames)

	assert.NoError(t, err)
	assert.Equal(t, expectedReturnUsers, users)
	m.AssertExpectations(t)
}
