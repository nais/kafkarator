package aiven

import (
	"fmt"
	"net/http"
)

type CreateUserRequest struct {
	Username       string `json:"username"`
	Authentication string `json:"authentication"`
}

type UserResponse struct {
	AccessCert                  string `json:"access_cert"`
	AccessCertNotValidAfterTime string `json:"access_cert_not_valid_after_time"`
	AccessKey                   string `json:"access_key"`
	Authentication              string `json:"authentication"`
	Password                    string `json:"password"`
	Type                        string `json:"type"`
	Username                    string `json:"username"`
}

func (c *Client) CreateUser(username string) (*UserResponse, error) {
	url := fmt.Sprintf("https://api.aiven.io/v1/project/%s/service/%s/user", c.Project, c.Service)
	req := CreateUserRequest{
		Username:       username,
		Authentication: "caching_sha2_password",
	}

	request, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}

	response, err := c.request(request, req)
	if err != nil {
		return nil, err
	}

	return response.User, nil
}

func (c *Client) DeleteUser(username string) error {
	url := fmt.Sprintf("https://api.aiven.io/v1/project/%s/service/%s/user/%s", c.Project, c.Service, username)

	request, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	_, err = c.request(request, nil)

	return err
}
