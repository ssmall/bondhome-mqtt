FROM golang:1.13-alpine AS builder

RUN mkdir -p /home/bondhome-mqtt/

COPY . /home/bondhome-mqtt/

WORKDIR /home/bondhome-mqtt/

RUN go build -v ./main.go

FROM alpine:3

RUN mkdir -p /home/bondhome-mqtt/

COPY --from=builder /home/bondhome-mqtt/main /home/bondhome-mqtt/

WORKDIR /home/bondhome-mqtt/

CMD ["./main", "-broker", "${BROKER}", "-bridge", "${BRIDGE}", "-token", "${TOKEN}"]