package serviceuser_test

import (
	"testing"

	"github.com/aiven/aiven-go-client"
	kafka_nais_io_v1 "github.com/nais/kafkarator/api/v1"
	"github.com/nais/kafkarator/pkg/aiven/serviceuser"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	project = "myproject"
	service = "myservice"
)

type ServiceUsersMock struct {
	mock.Mock
}

func (s *ServiceUsersMock) Create(project, service string, req aiven.CreateServiceUserRequest) (*aiven.ServiceUser, error) {
	args := s.Called(project, service, req)
	return args.Get(0).(*aiven.ServiceUser), args.Error(1)
}

func (s *ServiceUsersMock) List(project, serviceName string) ([]*aiven.ServiceUser, error) {
	args := s.Called(project, serviceName)
	return args.Get(0).([]*aiven.ServiceUser), args.Error(1)
}

func TestManager_Synchronize(t *testing.T) {
	m := &ServiceUsersMock{}

	usernames := make([]kafka_nais_io_v1.User, 0)
	usernames = append(usernames, kafka_nais_io_v1.User{
		Username:    "app1-team1",
		Application: "app1",
		Team:        "team1",
	})
	usernames = append(usernames, kafka_nais_io_v1.User{
		Username:    "app2-team1",
		Application: "app2",
		Team:        "team1",
	})

	req := aiven.CreateServiceUserRequest{
		Username: "app2-team1",
	}

	existingUsers := []*aiven.ServiceUser{
		{
			Username: "app1-team1",
		},
	}

	expectedCreateArgs := &aiven.ServiceUser{
		Username: "app2-team1",
	}

	m.On("Create", project, service, req).Return(expectedCreateArgs, nil)
	m.On("List", project, service).Return(existingUsers, nil)

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
