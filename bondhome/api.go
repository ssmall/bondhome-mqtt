package bondhome

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

// Device represents information about the device
// retrieved via the following API request: http://docs-local.appbond.com/#tag/Devices/paths/~1v2~1devices~1{device_id}/get
type Device struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Location string   `json:"location"`
	Actions  []string `json:"actions"`
}

// Bridge interface is used to communicate with the Bond bridge
type Bridge interface {
	ExecuteAction(deviceID string, actionID string, argumentJSON string) error
	GetDevice(deviceID string) (*Device, error)
	GetDeviceIDs() ([]string, error)
}

// NewBridge creates a new BondHome bridge API client
func NewBridge(hostname string, token string) Bridge {
	return &restAPIClient{
		client:   http.DefaultClient,
		hostname: hostname,
		token:    token,
	}
}

type restAPIClient struct {
	client   *http.Client
	hostname string
	token    string
}

type executeActionArg struct {
	Argument interface{} `json:"argument"`
}

func (c *restAPIClient) ExecuteAction(deviceID string, actionID string, argumentJSON string) error {
	req, err := c.newRequest(http.MethodPut, fmt.Sprintf("v2/devices/%s/actions/%s", deviceID, actionID), []byte(argumentJSON))
	if err != nil {
		return err
	}

	log.Printf("Sending request: %s %s body=%q", req.Method, req.URL, argumentJSON)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("error executing HTTP request: %w", err)
	}

	if err = expect2xxResponse(resp); err != nil {
		return err
	}

	return nil
}

func (c *restAPIClient) GetDevice(deviceID string) (*Device, error) {
	req, err := c.newRequest(http.MethodGet, "v2/devices/"+deviceID, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing HTTP request: %w", err)
	}

	defer resp.Body.Close()

	if err = expect2xxResponse(resp); err != nil {
		return nil, err
	}

	deviceResult := &Device{}

	err = unmarshalResponseBody(resp, deviceResult)

	if err != nil {
		return nil, err
	}

	return deviceResult, err
}

func (c *restAPIClient) GetDeviceIDs() ([]string, error) {
	req, err := c.newRequest(http.MethodGet, "v2/devices", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing HTTP request: %w", err)
	}

	defer resp.Body.Close()

	if err = expect2xxResponse(resp); err != nil {
		return nil, err
	}
	var responseObject map[string]interface{}

	err = unmarshalResponseBody(resp, &responseObject)

	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("failed to create request: %w", err)
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

func unmarshalResponseBody(r *http.Response, v interface{}) error {
	bodyBytes, err := ioutil.ReadAll(r.Body)

	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	err = json.Unmarshal(bodyBytes, v)

	if err != nil {
		return fmt.Errorf("error unmarshaling JSON from response body: %w\nBody: %s", err, bodyBytes)
	}
	return nil
}
