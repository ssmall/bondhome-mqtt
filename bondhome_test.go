package bondhome

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_restAPIClient_executeAction(t *testing.T) {
	deviceID := "testDeviceId"
	actionID := "testActionId"
	// use float64 for numeric value, per https://golang.org/pkg/encoding/json/#Unmarshal
	expectedArg := []interface{}{"1", float64(2), "three"}
	token := "testToken"
	expectedURLPath := fmt.Sprintf("/v2/devices/%s/actions/%s", deviceID, actionID)

	// Use channel to signal that a request was received
	received := make(chan int)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// close received channel so that test will panic if there is
		// more than one request
		close(received)
		receivedToken := r.Header.Get("BOND-Token")
		if receivedToken != token {
			t.Errorf("expected token header %q but got %q", token, receivedToken)
		}
		if r.Method != http.MethodPut {
			t.Errorf("expected request method %q but got %q", http.MethodPut, r.Method)
		}
		if r.URL.Path != expectedURLPath {
			t.Errorf("expected URL path %q but got %q", expectedURLPath, r.URL.Path)
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
	}))
	defer ts.Close()

	client := &restAPIClient{
		client:   ts.Client(),
		hostname: ts.URL,
		token:    token,
	}

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
