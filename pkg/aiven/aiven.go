package aiven

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

type Client struct {
	Project string
	Service string
	Token   string
}

type Error struct {
	Message  string `json:"message"`
	MoreInfo string `json:"more_info"`
	Status   int    `json:"status"`
}

type Response struct {
	Errors  Errors         `json:"errors"`
	Message string         `json:"message"`
	Topic   *TopicResponse `json:"topic,omitempty"`
	User    *UserResponse  `json:"user,omitempty"`
	ACL     []ACL          `json:"acl,omitempty"`
}

type Errors []Error

func (errors Errors) String() string {
	if len(errors) == 1 {
		return errors[0].Message
	}
	messages := make([]string, len(errors))
	for i := range errors {
		messages[i] = fmt.Sprintf("%d) %s", i+1, errors[i].Message)
	}
	return strings.Join(messages, "; ")
}

// send a generic HTTP request to Aiven with credentials and optionally a body.
// parse the response, and return a Response object.
func (c *Client) request(req *http.Request, data interface{}) (*Response, error) {
	if data != nil {
		body, err := json.Marshal(data)

		if err != nil {
			return nil, fmt.Errorf("encoding JSON: %s", err)
		}

		reader := bytes.NewReader(body)
		req.Body = ioutil.NopCloser(reader)
	}

	token := fmt.Sprintf("aivenv1 %s", c.Token)
	req.Header.Add("authorization", token)

	httpResponse, err := http.DefaultClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("%s request to Aiven on %s: %s", req.Method, req.RequestURI, err)
	}

	response := &Response{}
	responseBody, err := ioutil.ReadAll(httpResponse.Body)

	if err != nil {
		return nil, fmt.Errorf("reading response from Aiven: %s", err)
	}

	err = json.Unmarshal(responseBody, response)

	if err != nil {
		return nil, fmt.Errorf("parse JSON response from Aiven: %s", err)
	}

	if len(response.Errors) > 0 {
		//noinspection GoErrorStringFormat
		return nil, fmt.Errorf("Aiven failed with %d errors: %s", len(response.Errors), response.Errors)
	}

	return response, nil
}
