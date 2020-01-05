package bondhome

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Bridge interface is used to communicate with the Bond bridge
type Bridge interface {
	executeAction(deviceID string, actionID string) error
}

type restAPIClient struct {
	client   *http.Client
	hostname string
	token    string
}

type executeActionArg struct {
	Argument interface{} `json:"argument"`
}

func (c *restAPIClient) executeAction(deviceID string, actionID string, argument interface{}) error {
	requestArg := executeActionArg{argument}
	requestBody, err := json.Marshal(requestArg)
	if err != nil {
		return fmt.Errorf("error marshaling request body to JSON: %v", err)
	}

	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/v2/devices/%s/actions/%s", c.hostname, deviceID, actionID),
		bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Add("BOND-Token", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("error executing HTTP request: %v", err)
	}

	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return fmt.Errorf("expected 2xx response but got: %v", resp)
	}

	return nil
}
