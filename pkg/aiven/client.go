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

type Response struct {
	Errors  []Error `json:"errors"`
	Message string  `json:"message"`
}

type Config map[string]interface{}

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

func (c *Client) CreateTopic(project string, service string, req CreateTopicRequest) error {
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
