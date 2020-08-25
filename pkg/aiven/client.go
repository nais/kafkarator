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
	Token string
}

type CreateTopicRequest struct {
	Config      Config `json:"config"`
	TopicName   string `json:"topic_name"`
	Partitions  int    `json:"partitions"`
	Replication int    `json:"replication"`
}

type Error struct {
	Message  string `json:"message"`
	MoreInfo string `json:"more_info"`
	Status   int    `json:"status"`
}

type TopicResponse struct {
	Config      ConfigResponse      `json:"config"`
	Partitions  []PartitionResponse `json:"partitions,omitempty"`
	Replication int                 `json:"replication"`
}

type PartitionResponse struct {
}

type Errors []Error

type Response struct {
	Errors  Errors         `json:"errors"`
	Message string         `json:"message"`
	Topic   *TopicResponse `json:"topic,omitempty"`
}

type Config map[string]interface{}

type ConfigResponse map[string]ConfigResponseDescriptor

type ConfigResponseDescriptor struct {
	Value interface{} `json:"value"`
}

var defaultConfig = Config{
	"retention_ms": 1209600000, // two weeks
}

func WithDefaultConfig(conf Config) Config {
	merged := make(Config)
	for k, v := range defaultConfig {
		merged[k] = v
	}
	for k, v := range conf {
		merged[k] = v
	}
	return merged
}

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

func (c *Client) CreateTopic(project, service string, req CreateTopicRequest) error {
	req.Config = WithDefaultConfig(req.Config)

	url := fmt.Sprintf("https://api.aiven.io/v1/project/%s/service/%s/topic", project, service)

	request, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	_, err = c.request(request, req)

	return err
}

func (c *Client) GetTopic(project, service, topic string) (*TopicResponse, error) {
	url := fmt.Sprintf("https://api.aiven.io/v1/project/%s/service/%s/topic/%s", project, service, topic)

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	response, err := c.request(request, nil)
	if err != nil {
		return nil, err
	}

	return response.Topic, nil
}

func (c *Client) DeleteTopic(project, service, topic string) error {
	url := fmt.Sprintf("https://api.aiven.io/v1/project/%s/service/%s/topic/%s", project, service, topic)

	request, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	_, err = c.request(request, nil)

	return err
}
