# bondhome-mqtt
MQTT bridge for BondHome API. See http://docs-local.appbond.com

## Overview

This program does two things:
1. Relay commands received via MQTT to the Bond Bridge API
2. Update MQTT topics with device status (subscribed via BPUP<sup>[1]</sup>)

### Topics

On startup, the `bondhome-mqtt` program gets a list of all devices connected
to the Bond Home Bridge and sets up the following MQTT topics for each device:

`bondhome/devices/<device id>/<action>` for triggering actions

`bondhome/devices/<device id>/state` for publishing device state

## Usage

### Command line

```bash
go run main.go -broker tcp://<host>:<port> -bridge <ip> -token <token>
```

#### Options

*  `-broker` the address of the MQTT broker, in the form `tcp://<host>:<port>`
*  `-bridge` the IP address of the Bond bridge
*  `-token` the Bond API token, see [2] for instructions on getting the correct value
*  `-logtostderr` enables additional logging output (by default, only warnings and errors will be logged)
*  `-v=N` enables verbose logging at level `N`

### Docker

A pre-built Docker image is available: `docker pull docker pull ghcr.io/ssmall/bondhome-mqtt:v1.0.0`

[1]: http://docs-local.appbond.com/#section/Bond-Push-UDP-Protocol-(BPUP)
[2]: http://docs-local.appbond.com/#section/Getting-Started/Getting-the-Bond-Token