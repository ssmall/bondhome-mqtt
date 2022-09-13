package mqtt

import (
	"fmt"
	"os"
	"time"

	"github.com/golang/glog"

	paho "github.com/eclipse/paho.mqtt.golang"
)

const (
	connectTimeout = 10 * time.Second
)

// NewClient creates a new MQTT client and tries to establish
// a connection to the specified broker
func NewClient(broker string) (paho.Client, error) {
	clientID, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	glog.Infof("Establishing connection to MQTT broker @ %s using client ID %q", broker, clientID)
	opts := paho.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	client := paho.NewClient(opts)
	connectToken := client.Connect()
	if !connectToken.WaitTimeout(connectTimeout) {
		return nil, fmt.Errorf("timed out after %v", connectTimeout)
	}
	return client, nil
}
