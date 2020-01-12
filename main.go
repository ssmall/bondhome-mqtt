package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/ssmall/bondhome-mqtt/bondhome"
	"github.com/ssmall/bondhome-mqtt/mqtt"
	"golang.org/x/sync/errgroup"

	paho "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	brokerAddress := flag.String("broker", "", "The broker to connect to; see https://godoc.org/github.com/eclipse/paho.mqtt.golang#ClientOptions.AddBroker")
	bridgeAddress := flag.String("bridge", "", "The hostname or IP address of the Bond Home bridge")
	bridgeToken := flag.String("token", "", "The Bond Home bridge API token. See http://docs-local.appbond.com/#section/Getting-Started/Getting-the-Bond-Token")
	flag.Parse()

	if *brokerAddress == "" {
		log.Fatal("Must specify broker!")
	}
	if *bridgeAddress == "" {
		log.Fatal("Must specify bridge!")
	}
	if *bridgeToken == "" {
		log.Fatal("Must specify token!")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mqttClient, err := mqtt.NewClient(*brokerAddress)

	if err != nil {
		log.Fatalf("Unable to connect to MQTT broker: %v", err)
	}

	log.Println("Connected to broker @ ", *brokerAddress)

	bridge := bondhome.NewBridge(*bridgeAddress, *bridgeToken)

	err = setupDeviceActionHandlers(ctx, bridge, mqttClient)
	if err != nil {
		log.Fatal("Exiting due to error:", err)
	}

	pushClient, err := bondhome.NewClient(ctx, *bridgeAddress+":30007")
	if err != nil {
		log.Fatal("Exiting due to error:", err)
	}

	err = setupDeviceStateHandlers(ctx, pushClient, mqttClient)
	if err != nil {
		log.Fatal("Exiting due to error:", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	s := <-c
	log.Printf("Got %s, exiting", s)
}

func setupDeviceStateHandlers(ctx context.Context, pushClient bondhome.PushClient, mqttClient paho.Client) error {
	err := pushClient.StartListening()
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				update, err := pushClient.Receive(10 * time.Second)
				if err != nil {
					if e, ok := err.(bondhome.Timeout); !ok {
						panic(fmt.Errorf("error receiving from Bond Bridge: %w", e))
					}
				}
				if update != nil && update.Topic != "" {
					topic := "bondhome/" + update.Topic
					body, err := update.Body.MarshalJSON()
					if err != nil {
						log.Println("Unable to marshal update body to JSON", err)
					}
					log.Printf("Publishing to %s with body: %v", topic, string(body))
					token := mqttClient.Publish(topic, byte(0), false, string(body))
					if token.Wait() && token.Error() != nil {
						log.Printf("Unable to publish to topic %s: %v", topic, token.Error())
					}
				} else if update != nil && update.ErrorMsg != "" {
					log.Printf("Got error response from Bond Home bridge: code %d %q", update.ErrorID, update.ErrorMsg)
				}
			}
		}
	}()

	return nil
}

func setupDeviceActionHandlers(ctx context.Context, bridge bondhome.Bridge, mqttClient paho.Client) error {
	devices, err := bridge.GetDeviceIDs()

	if err != nil {
		return fmt.Errorf("could not get devices from bridge: %w", err)
	}

	var g errgroup.Group

	for _, deviceID := range devices {
		localDeviceID := deviceID
		g.Go(func() error {
			d, err := bridge.GetDevice(localDeviceID)
			if err != nil {
				return err
			}
			log.Printf("Discovered device with id %q: %#v", localDeviceID, d)

			var hg errgroup.Group

			for _, actionID := range d.Actions {
				localActionID := actionID
				hg.Go(func() error {
					return actionHandler(mqttClient, bridge, localDeviceID, localActionID)
				})
			}

			return hg.Wait()
		})
	}

	err = g.Wait()
	if err != nil {
		return fmt.Errorf("error setting up listeners: %w", err)
	}
	return nil
}

func actionHandler(mqtt paho.Client, bridge bondhome.Bridge, deviceID string, actionID string) error {
	topic := fmt.Sprintf("bondhome/devices/%s/%s", deviceID, actionID)

	token := mqtt.Subscribe(topic, byte(0), func(c paho.Client, m paho.Message) {
		log.Printf("Message(%d): %q on topic %s", m.MessageID(), m.Payload(), m.Topic())
		if err := bridge.ExecuteAction(deviceID, actionID, string(m.Payload())); err != nil {
			log.Printf("Not acking message due to error executing action: %v\n", err)
		} else {
			m.Ack()
		}
	})

	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("unable to subscribe to topic %s: %w", topic, token.Error())
	}

	log.Println("Subcribed to topic", topic)

	return nil
}
