package aiven

import (
	"fmt"
	"net/http"
)

type ACL struct {
	ID         string `json:"id,omitempty"`
	Permission string `json:"permission"`
	Topic      string `json:"topic"`
	Username   string `json:"username"`
}

func (c *Client) CreateACL(username, topic, permission string) ([]ACL, error) {
	url := fmt.Sprintf("https://api.aiven.io/v1/project/%s/service/%s/acl", c.Project, c.Service)
	req := ACL{
		Permission: permission,
		Topic:      topic,
		Username:   username,
	}

	request, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}

	response, err := c.request(request, req)
	if err != nil {
		return nil, err
	}

	return response.ACL, nil
}

func (c *Client) DeleteACL(id string) error {
	url := fmt.Sprintf("https://api.aiven.io/v1/project/%s/service/%s/acl/%s", c.Project, c.Service, id)

	request, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	_, err = c.request(request, nil)

	return err
}
