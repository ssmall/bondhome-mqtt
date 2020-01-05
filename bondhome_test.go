package bondhome

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	deviceID             = "testDeviceId"
	actionID             = "testActionId"
	token                = "testToken"
	executeActionURLPath = "/v2/devices/" + deviceID + "/actions/" + actionID
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

func Test_restAPIClient_executeAction(t *testing.T) {
	// use float64 for numeric value, per https://golang.org/pkg/encoding/json/#Unmarshal
	expectedArg := []interface{}{"1", float64(2), "three"}

	ts, client, received := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedToken := r.Header.Get("BOND-Token")
		if receivedToken != token {
			t.Errorf("expected token header %q but got %q", token, receivedToken)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected request method %q but got %q", http.MethodPut, r.Method)
		}
		if r.URL.Path != executeActionURLPath {
			t.Errorf("expected URL path %q but got %q", executeActionURLPath, r.URL.Path)
		}
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

	err := client.executeAction(deviceID, actionID, expectedArg)
	if err != nil {
		t.Errorf("got error: %v", err)
	}

	select {
	case _, ok := <-received:
		if ok {
			t.Fatal("Received a value but channel was not closed. Wtf?")
		}
	default:
		t.Fatal("No request was sent")
	}
}

func Test_restAPIClient_executeAction_serverError(t *testing.T) {
	ts, client, received := setupTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "expected error", 503)
	})
	defer ts.Close()

	err := client.executeAction(deviceID, actionID, nil)

	select {
	case _, ok := <-received:
		if ok {
			t.Fatal("Received a value but channel was not closed. Wtf?")
		}
	default:
		t.Fatal("No request was sent")
	}

	if err == nil {
		t.Errorf("expected an error but got none")
	}
}
