package bondhome

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

const (
	deviceID = "testDeviceId"
	actionID = "testActionId"
	token    = "testToken"
)

func setupTestServer(t *testing.T, requestHandler func(http.ResponseWriter, *http.Request)) (*httptest.Server, *restAPIClient, <-chan (int)) {
	t.Helper()
	// Use channel to signal that a request was received
	received := make(chan int)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// close received channel so that test will panic if there is
		// more than one request
		close(received)
		requestHandler(w, r)
	}))

	client := &restAPIClient{
		client:   ts.Client(),
		hostname: ts.URL,
		token:    token,
	}

	return ts, client, received
}

func expectRequestReceived(t *testing.T, received <-chan (int)) {
	t.Helper()
	select {
	case _, ok := <-received:
		if ok {
			t.Fatal("Received a value but channel was not closed. Wtf?")
		}
	default:
		t.Fatal("No request was sent")
	}
}

func expectToken(t *testing.T, r *http.Request) {
	t.Helper()
	receivedToken := r.Header.Get("BOND-Token")
	if receivedToken != token {
		t.Errorf("expected token header %q but got %q", token, receivedToken)
	}
}

func expectMethod(t *testing.T, expectedMethod string, r *http.Request) {
	t.Helper()
	if r.Method != expectedMethod {
		t.Errorf("expected request method %q but got %q", expectedMethod, r.Method)
	}
}

func expectURLPath(t *testing.T, expectedPath string, r *http.Request) {
	if r.URL.Path != expectedPath {
		t.Errorf("expected URL path %q but got %q", expectedPath, r.URL.Path)
	}
}

func Test_restAPIClient_executeAction(t *testing.T) {
	// use float64 for numeric value, per https://golang.org/pkg/encoding/json/#Unmarshal
	expectedArg := []interface{}{"1", float64(2), "three"}

	ts, client, received := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		expectToken(t, r)
		expectMethod(t, http.MethodPut, r)
		expectURLPath(t, "/v2/devices/"+deviceID+"/actions/"+actionID, r)
		defer r.Body.Close()
		if bodyBytes, err := ioutil.ReadAll(r.Body); err != nil {
			t.Errorf("error reading request body: %v", err)
		} else {
			var requestBody struct {
				Argument interface{} `json:"argument"`
			}
			if err = json.Unmarshal(bodyBytes, &requestBody); err != nil {
				t.Errorf("error unmarshalling request body: %v\nBody: %s", err, bodyBytes)
			} else {
				switch actualArg := requestBody.Argument.(type) {
				case []interface{}:
					if len(actualArg) != len(expectedArg) {
						t.Errorf("expected argument %q, but got %q", expectedArg, actualArg)
					}
					for i, actual := range actualArg {
						if actual != expectedArg[i] {
							t.Errorf("expected argument <%[1]v> (type %[1]T) at index %[2]d but got argument <%[3]v> (type %[3]T)", expectedArg[i], i, actual)
						}
					}
				default:
					t.Errorf("expected argument to have type %T, but got %T", expectedArg, actualArg)
				}
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer ts.Close()

	err := client.ExecuteAction(deviceID, actionID, expectedArg)
	if err != nil {
		t.Errorf("got error: %v", err)
	}

	expectRequestReceived(t, received)
}

func Test_restAPIClient_executeAction_serverError(t *testing.T) {
	ts, client, received := setupTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "expected error", 503)
	})
	defer ts.Close()

	err := client.ExecuteAction(deviceID, actionID, nil)

	expectRequestReceived(t, received)

	if err == nil {
		t.Errorf("expected an error but got none")
	}

	if !strings.Contains(err.Error(), "expected 2xx response but got") {
		t.Fatalf("got different error than expected: %v", err)
	}
}

func Test_restAPIClient_getDeviceIds(t *testing.T) {
	expectedDeviceIDs := map[string]bool{
		"deviceID1": true,
		"deviceID2": true,
		"deviceID3": true,
	}
	ts, client, received := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		expectToken(t, r)
		expectMethod(t, http.MethodGet, r)
		expectURLPath(t, "/v2/devices", r)

		const responseJSON = `{
			"_": "7fc1e84b",
			"deviceID1": {
				"_": "9a5e1136"
			},
			"deviceID2": {
				"_": "409d124b"
			},
			"deviceID3": {
				"_": "aaf392f2"
			}
		}`

		w.Write([]byte(responseJSON))
	})
	defer ts.Close()

	actualIDs, err := client.GetDeviceIDs()

	if err != nil {
		t.Fatalf("unexpected request error: %v", err)
	}

	expectRequestReceived(t, received)

	for _, v := range actualIDs {
		if _, ok := expectedDeviceIDs[v]; !ok {
			t.Errorf("unexpected device ID: %q", v)
		}
	}

	if len(actualIDs) != len(expectedDeviceIDs) {
		t.Fatalf("expected response containing %v but got %v", expectedDeviceIDs, actualIDs)
	}
}

func Test_restAPIClient_getDeviceIds_serverError(t *testing.T) {
	ts, client, received := setupTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "expected error", 503)
	})
	defer ts.Close()

	_, err := client.GetDeviceIDs()

	expectRequestReceived(t, received)

	if err == nil {
		t.Fatalf("expected an error but got none")
	}

	if !strings.Contains(err.Error(), "expected 2xx response but got") {
		t.Fatalf("got different error than expected: %v", err)
	}
}

func Test_restAPIClient_getDevice(t *testing.T) {
	expectedDevice := &Device{
		Name:     "Fireplace",
		Type:     "FP",
		Location: "Living Room",
		Actions: []string{
			"TurnOff",
			"TurnOn",
			"Stop",
			"TogglePower",
		},
	}

	ts, client, received := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		expectToken(t, r)
		expectMethod(t, http.MethodGet, r)
		expectURLPath(t, "/v2/devices/"+deviceID, r)

		const responseJSON = `{
			"name": "Fireplace",
			"type": "FP",
			"location": "Living Room",
			"actions": [
				"TurnOff",
				"TurnOn",
				"Stop",
				"TogglePower"
			],
			"_": "54d9b20a",
			"commands": {
				"_": "7618645a"
			},
			"state": {
				"_": "fe8c688d"
			},
			"properties": {
				"_": "598b1c06"
			}
		}`

		w.Write([]byte(responseJSON))
	})
	defer ts.Close()

	d, err := client.GetDevice(deviceID)

	if err != nil {
		t.Fatalf("unexpected request error: %v", err)
	}

	expectRequestReceived(t, received)

	if !reflect.DeepEqual(d, expectedDevice) {
		t.Fatalf("expected:\n%#v\nbut was:\n%#v", *expectedDevice, *d)
	}
}

func Test_restAPIClient_getDevice_serverError(t *testing.T) {
	ts, client, received := setupTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "expected error", 503)
	})
	defer ts.Close()

	_, err := client.GetDevice(deviceID)

	expectRequestReceived(t, received)

	if err == nil {
		t.Fatalf("expected an error but got none")
	}

	if !strings.Contains(err.Error(), "expected 2xx response but got") {
		t.Fatalf("got different error than expected: %v", err)
	}
}
