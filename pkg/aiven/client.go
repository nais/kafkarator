package aiven

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Client struct {
	token string
}

type CreateTopicPayload struct {
	Config      map[string]string
	TopicName   string `json:"topic_name"`
	Partitions  int
	Replication int
}

type Error struct {
	Message  string
	MoreInfo string `json:"more_info"`
	Status   int
}

type Response struct {
	Errors  []Error
	Message string
}

func (c *Client) createTopic(project string, service string, payload CreateTopicPayload) error {
	url := fmt.Sprintf("https://api.aiven.io/v1/project/%s/service/%s/topic", project, service)
	body, err := json.Marshal(payload)

	if err != nil {
		return err
	}

	reader := bytes.NewReader(body)
	request, err := http.NewRequest("POST", url, reader)

	if err != nil {
		return err
	}

	token := fmt.Sprintf("aivenv1 %s", c.token)
	request.Header.Add("authorization", token)
	httpResponse, err := http.DefaultClient.Do(request)

	if err != nil {
		return err
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
		return fmt.Errorf(response.Message)
	}

	return nil
}
