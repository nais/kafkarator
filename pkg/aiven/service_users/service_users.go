package service_users

import (
	"github.com/aiven/aiven-go-client"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UserReconciler struct {
	Aiven   *aiven.Client
	K8s     client.Client
	Project string
	Service string
}

func (r *UserReconciler) UpdateUsers(users []string) error {
	missing, err := r.findMissingServiceUsers(users)
	if err != nil {
		return err
	}

	err = r.createServiceUsers(missing)
	if err != nil {
		return err
	}
	return nil
}

func (r *UserReconciler) createServiceUsers(missing []string) error {
	for _, user := range missing {
		req := aiven.CreateServiceUserRequest{user}
		serviceUser, err := r.Aiven.ServiceUsers.Create(r.Project, r.Service, req)
		if err != nil {
			return err
		}
		err = r.CreateKubernetesSecret(serviceUser)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *UserReconciler) findMissingServiceUsers(users []string) ([]string, error) {
	serviceUsers, err := r.Aiven.ServiceUsers.List(r.Project, r.Service)
	if err != nil {
		return nil, err
	}
	serviceUserMap := make(map[string]bool, len(serviceUsers))
	for _, serviceUser := range serviceUsers {
		serviceUserMap[serviceUser.Username] = true
	}
	result := make([]string, 0, len(users))
	for _, user := range users {
		if !serviceUserMap[user] {
			result = append(result, user)
		}
	}
	return result, nil
}

func (r *UserReconciler) CreateKubernetesSecret(user *aiven.ServiceUser) error {
	// TODO:
	return nil
}
