package serviceuser

import (
	"fmt"
	"strings"

	"github.com/aiven/aiven-go-client"
	"github.com/nais/kafkarator/pkg/utils"
	log "github.com/sirupsen/logrus"
)

const (
	MaxServiceUserNameLength = 40
)

type UserMap struct {
	Name      string
	AivenName string
	User      *aiven.ServiceUser
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

func AivenUserName(username string) string {
	username, _ = utils.ShortName(username, MaxServiceUserNameLength)
	return username
}

func mapUsers(usernames []string) []*UserMap {
	mp := make([]*UserMap, len(usernames))
	for i := range usernames {
		mp[i] = &UserMap{
			Name:      usernames[i],
			AivenName: AivenUserName(usernames[i]),
		}
	}
	return mp
}

// Create Aiven users from a list of usernames, if they are not found on the Aiven service.
// Returns a list of all users, both created and existing.
func (r *Manager) Synchronize(usernames []string) ([]*UserMap, error) {
	userMap := mapUsers(usernames)

	serviceUsers, err := r.AivenServiceUsers.List(r.Project, r.Service)
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

func NameParts(username string) (team, application string, err error) {
	s := strings.SplitN(username, "__", 2)
	if len(s) != 2 {
		return "", "", fmt.Errorf("unable to parse team and application name from username %s", username)
	}
	return s[0], s[1], nil
}

func (r *Manager) createServiceUsers(users []*UserMap) error {
	var err error

	for i, user := range users {
		logger := r.Logger.WithFields(log.Fields{
			"username": user.AivenName,
		})
		if user.User != nil {
			logger.Infof("Skip creating service user")
			continue
		}

		logger.Infof("Creating service user")

		req := aiven.CreateServiceUserRequest{
			Username: user.AivenName,
		}

		users[i].User, err = r.AivenServiceUsers.Create(r.Project, r.Service, req)
		if err != nil {
			return err
		}
	}

	return nil
}

func populate(users []*UserMap, serviceUsers []*aiven.ServiceUser) {
	for _, existingUser := range serviceUsers {
		for _, user := range users {
			if user.AivenName == existingUser.Username {
				user.User = existingUser
			}
		}
	}
}
