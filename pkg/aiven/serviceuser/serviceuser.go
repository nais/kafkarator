package serviceuser

import (
	"fmt"
	"strings"

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

// Create Aiven users from a list of usernames, if they are not found on the Aiven service.
// Returns a list of all users, both created and existing.
func (r *Manager) Synchronize(users []string) (map[string]*aiven.ServiceUser, error) {
	serviceUsers, err := r.AivenServiceUsers.List(r.Project, r.Service)
	if err != nil {
		return nil, fmt.Errorf("unable to list service users: %s", err)
	}

	missing, err := r.findMissingServiceUsers(users, serviceUsers)

	if err != nil {
		return nil, err
	}

	createdUsers, err := r.createServiceUsers(missing)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*aiven.ServiceUser)
	for _, user := range serviceUsers {
		result[user.Username] = user
	}
	for _, user := range createdUsers {
		result[user.Username] = user
	}

	return result, nil
}

func NameParts(username string) (team, application string, err error) {
	s := strings.SplitN(username, "__", 2)
	if len(s) != 2 {
		return "", "", fmt.Errorf("unable to parse team and application name from username %s", username)
	}
	return s[0], s[1], nil
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

func (r *Manager) findMissingServiceUsers(users []string, serviceUsers []*aiven.ServiceUser) ([]string, error) {
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
