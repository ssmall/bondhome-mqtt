package mqtt

import (
	"fmt"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

const (
	clientID       = "bondhome-mqtt"
	connectTimeout = 10 * time.Second
)

// NewClient creates a new MQTT client and tries to establish
// a connection to the specified broker
func NewClient(broker string) (paho.Client, error) {
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
