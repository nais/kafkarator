package aiven

import (
	"fmt"
	"net/http"
)

type CreateTopicRequest struct {
	Config      Config `json:"config"`
	TopicName   string `json:"topic_name"`
	Partitions  int    `json:"partitions"`
	Replication int    `json:"replication"`
}

type TopicResponse struct {
	Config      ConfigResponse      `json:"config"`
	Partitions  []PartitionResponse `json:"partitions,omitempty"`
	Replication int                 `json:"replication"`
}

type PartitionResponse struct {
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

func (c *Client) CreateTopic(req CreateTopicRequest) error {
	req.Config = WithDefaultConfig(req.Config)

	url := fmt.Sprintf("https://api.aiven.io/v1/project/%s/service/%s/topic", c.Project, c.Service)

	request, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	_, err = c.request(request, req)

	return err
}

func (c *Client) GetTopic(topic string) (*TopicResponse, error) {
	url := fmt.Sprintf("https://api.aiven.io/v1/project/%s/service/%s/topic/%s", c.Project, c.Service, topic)

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

func (c *Client) DeleteTopic(topic string) error {
	url := fmt.Sprintf("https://api.aiven.io/v1/project/%s/service/%s/topic/%s", c.Project, c.Service, topic)

	request, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	_, err = c.request(request, nil)

	return err
}
