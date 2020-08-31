package serviceuser

import (
	"fmt"

	"github.com/aiven/aiven-go-client"
	log "github.com/sirupsen/logrus"
)

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

// given a list of usernames, create Aiven users not found in that list
func (r *Manager) Synchronize(users []string) ([]*aiven.ServiceUser, error) {
	missing, err := r.findMissingServiceUsers(users)

	if err != nil {
		return nil, err
	}

	return r.createServiceUsers(missing)
}

func (r *Manager) createServiceUsers(missing []string) ([]*aiven.ServiceUser, error) {
	var err error

	users := make([]*aiven.ServiceUser, len(missing))

	for i, user := range missing {
		req := aiven.CreateServiceUserRequest{
			Username: user,
		}

		users[i], err = r.AivenServiceUsers.Create(r.Project, r.Service, req)
		if err != nil {
			return nil, err
		}

		r.Logger.WithFields(log.Fields{
			"username": user,
		}).Infof("Created service user")
	}

	return users, nil
}

func (r *Manager) findMissingServiceUsers(users []string) ([]string, error) {
	serviceUsers, err := r.AivenServiceUsers.List(r.Project, r.Service)
	if err != nil {
		return nil, fmt.Errorf("unable to list service users: %s", err)
	}

	serviceUserMap := make(map[string]bool, len(serviceUsers))

	for _, user := range users {
		serviceUserMap[user] = false
	}

	for _, serviceUser := range serviceUsers {
		serviceUserMap[serviceUser.Username] = true
	}

	result := make([]string, 0, len(users))
	for user, exists := range serviceUserMap {
		if !exists {
			result = append(result, user)
		}
	}

	return result, nil
}
