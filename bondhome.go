package bondhome

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// Bridge interface is used to communicate with the Bond bridge
type Bridge interface {
	ExecuteAction(deviceID string, actionID string) error
	GetDeviceIDs() ([]string, error)
}

type restAPIClient struct {
	client   *http.Client
	hostname string
	token    string
}

type executeActionArg struct {
	Argument interface{} `json:"argument"`
}

func (c *restAPIClient) ExecuteAction(deviceID string, actionID string, argument interface{}) error {
	requestArg := executeActionArg{argument}
	requestBody, err := json.Marshal(requestArg)
	if err != nil {
		return fmt.Errorf("error marshaling request body to JSON: %v", err)
	}

	req, err := c.newRequest(http.MethodPut, fmt.Sprintf("v2/devices/%s/actions/%s", deviceID, actionID), requestBody)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("error executing HTTP request: %v", err)
	}

	if err = expect2xxResponse(resp); err != nil {
		return err
	}

	return nil
}

func (c *restAPIClient) GetDeviceIDs() ([]string, error) {
	req, err := c.newRequest(http.MethodGet, "v2/devices", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing HTTP request: %v", err)
	}

	defer resp.Body.Close()

	if err = expect2xxResponse(resp); err != nil {
		return nil, err
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	var responseObject map[string]interface{}

	err = json.Unmarshal(bodyBytes, &responseObject)

	if err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON from response body: %v\nBody: %s", err, bodyBytes)
	}

	ids := make([]string, 0, len(responseObject)-1)

	for k := range responseObject {
		if k != "_" {
			ids = append(ids, k)
		}
	}

	return ids, nil
}

func (c *restAPIClient) newRequest(method string, urlPath string, body []byte) (*http.Request, error) {
	req, err := http.NewRequest(method,
		fmt.Sprintf("%s/%s", c.hostname, urlPath),
		bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Add("BOND-Token", c.token)

	return req, nil
}

func expect2xxResponse(r *http.Response) error {
	if !(r.StatusCode >= 200 && r.StatusCode < 300) {
		return fmt.Errorf("expected 2xx response but got: %v", r)
	}
	return nil
}
