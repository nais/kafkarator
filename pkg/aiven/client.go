package aiven

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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

type Response struct {
	Errors  []Error        `json:"errors"`
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

func (c *Client) CreateTopic(project, service string, req CreateTopicRequest) error {
	req.Config = WithDefaultConfig(req.Config)

	url := fmt.Sprintf("https://api.aiven.io/v1/project/%s/service/%s/topic", project, service)
	body, err := json.Marshal(req)

	if err != nil {
		return err
	}

	reader := bytes.NewReader(body)
	request, err := http.NewRequest("POST", url, reader)

	if err != nil {
		return err
	}

	token := fmt.Sprintf("aivenv1 %s", c.Token)
	request.Header.Add("authorization", token)
	httpResponse, err := http.DefaultClient.Do(request)

	if err != nil {
		return fmt.Errorf("POST request to Aiven: %s", err)
	}

	response := Response{}
	responseBody, err := ioutil.ReadAll(httpResponse.Body)

	if err != nil {
		return err
	}

	err = json.Unmarshal(responseBody, &response)

	if err != nil {
		return err
	}

	if len(response.Errors) > 0 {
		//noinspection GoErrorStringFormat
		return fmt.Errorf("Aiven failed with %d errors: %s", len(response.Errors), response.Message)
	}

	return nil
}

func (c *Client) GetTopic(project, service, topic string) (*TopicResponse, error) {
	url := fmt.Sprintf("https://api.aiven.io/v1/project/%s/service/%s/topic/%s", project, service, topic)

	request, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return nil, err
	}

	token := fmt.Sprintf("aivenv1 %s", c.Token)
	request.Header.Add("authorization", token)
	httpResponse, err := http.DefaultClient.Do(request)

	if err != nil {
		return nil, fmt.Errorf("GET request to Aiven: %s", err)
	}

	response := Response{}
	responseBody, err := ioutil.ReadAll(httpResponse.Body)

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(responseBody, &response)

	if err != nil {
		return nil, err
	}

	if len(response.Errors) > 0 {
		//noinspection GoErrorStringFormat
		return nil, fmt.Errorf("Aiven failed with %d errors: %s", len(response.Errors), response.Message)
	}

	return response.Topic, nil
}

func (c *Client) DeleteTopic(project, service, topic string) error {
	url := fmt.Sprintf("https://api.aiven.io/v1/project/%s/service/%s/topic/%s", project, service, topic)

	request, err := http.NewRequest("DELETE", url, nil)

	if err != nil {
		return err
	}

	token := fmt.Sprintf("aivenv1 %s", c.Token)
	request.Header.Add("authorization", token)
	httpResponse, err := http.DefaultClient.Do(request)

	if err != nil {
		return fmt.Errorf("DELETE request to Aiven: %s", err)
	}

	response := Response{}
	responseBody, err := ioutil.ReadAll(httpResponse.Body)

	if err != nil {
		return err
	}

	err = json.Unmarshal(responseBody, &response)

	if err != nil {
		return err
	}

	if len(response.Errors) > 0 {
		//noinspection GoErrorStringFormat
		return fmt.Errorf("Aiven failed with %d errors: %s", len(response.Errors), response.Message)
	}

	return nil
}
