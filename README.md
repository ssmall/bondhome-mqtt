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

See [2] for instructions on getting the correct value for `<token>`

### Docker

The pre-built image is available on Docker Hub as [spencersmall/bondhome-mqtt](https://hub.docker.com/r/spencersmall/bondhome-mqtt).

A [Dockerfile](Dockerfile) is also included in this repo.

[1]: http://docs-local.appbond.com/#section/Bond-Push-UDP-Protocol-(BPUP)
[2]: http://docs-local.appbond.com/#section/Getting-Started/Getting-the-Bond-Token