package serviceuser_test

import (
	"testing"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/pkg/aiven/serviceuser"
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

	usernames := []string{
		"user1",
		"user2",
	}

	req := aiven.CreateServiceUserRequest{
		Username: "user2",
	}

	existingUsers := []*aiven.ServiceUser{
		{
			Username: "user1",
		},
	}

	expectedCreateArgs := &aiven.ServiceUser{
		Username: "user2",
	}

	m.On("Create", project, service, req).Return(expectedCreateArgs, nil)
	m.On("List", project, service).Return(existingUsers, nil)

	manager := serviceuser.Manager{
		AivenServiceUsers: m,
		Project:           project,
		Service:           service,
	}

	expectedCreatedUsers := []*aiven.ServiceUser{
		{
			Username: "user2",
		},
	}
	users, err := manager.Synchronize(usernames)

	assert.NoError(t, err)
	assert.Equal(t, expectedCreatedUsers, users)
	m.AssertExpectations(t)
}
