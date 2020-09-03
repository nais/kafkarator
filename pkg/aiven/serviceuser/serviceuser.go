package serviceuser

import (
	"fmt"
	"github.com/nais/kafkarator/pkg/metrics"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/api/v1"
	log "github.com/sirupsen/logrus"
)

type UserMap struct {
	kafka_nais_io_v1.User
	AivenUser *aiven.ServiceUser
}

type ServiceUser interface {
	Create(project, service string, req aiven.CreateServiceUserRequest) (*aiven.ServiceUser, error)
	List(project, serviceName string) ([]*aiven.ServiceUser, error)
}

type Manager struct {
	AivenServiceUsers ServiceUser
	Project           string
	Service           string
	Logger            *log.Entry
}

func mapUsers(users []kafka_nais_io_v1.User) []*UserMap {
	mp := make([]*UserMap, len(users))
	for i := range users {
		mp[i] = &UserMap{
			User: users[i],
		}
	}
	return mp
}

// Create Aiven users from a list of usernames, if they are not found on the Aiven service.
// Returns a list of all users, both created and existing.
func (r *Manager) Synchronize(users []kafka_nais_io_v1.User) ([]*UserMap, error) {
	userMap := mapUsers(users)

	var serviceUsers []*aiven.ServiceUser
	err := metrics.ObserveAivenLatency("ServiceUser_List", r.Project, func() error {
		var err error
		serviceUsers, err = r.AivenServiceUsers.List(r.Project, r.Service)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("unable to list service users: %s", err)
	}

	populate(userMap, serviceUsers)

	err = r.createServiceUsers(userMap)
	if err != nil {
		return nil, err
	}

	return userMap, nil
}

func (r *Manager) createServiceUsers(users []*UserMap) error {
	var err error

	for i, user := range users {
		logger := r.Logger.WithFields(log.Fields{
			"username": user.Username,
		})
		if user.AivenUser != nil {
			logger.Infof("Skip creating service user")
			continue
		}

		logger.Infof("Creating service user")

		req := aiven.CreateServiceUserRequest{
			Username: user.Username,
		}

		err = metrics.ObserveAivenLatency("ServiceUser_Create", r.Project, func() error {
			var err error
			users[i].AivenUser, err = r.AivenServiceUsers.Create(r.Project, r.Service, req)
			return err
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func populate(users []*UserMap, serviceUsers []*aiven.ServiceUser) {
	for _, existingUser := range serviceUsers {
		for _, user := range users {
			if user.Username == existingUser.Username {
				user.AivenUser = existingUser
			}
		}
	}
}
