package bondhome

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"
)

// Update represents an update message from the Bond Bridge
type Update struct {
	BondID     string      `json:"B"`
	Topic      string      `json:"t"`
	StatusCode int         `json:"s"`
	HTTPMethod uint8       `json:"m"`
	Body       interface{} `json:"b"`

	// Error fields
	ErrorID  int    `json:"err_id"`
	ErrorMsg string `json:"err_msg"`
}

// PushClient is an interface for receiving messages
// pushed from a Bond Home bridge
type PushClient interface {
	// StartListening initiates a connection to listen for
	// updates from the Bond Home bridge, using the supplied
	// context for the signal to stop listening.
	StartListening(ctx context.Context) error
}

type bpupClient struct {
	conn *net.UDPConn
}

// NewClient creates a new PushClient that receives updates
// from the bridge at the given address
func NewClient(bridgeAddress string) (PushClient, error) {
	addr, err := net.ResolveUDPAddr("udp", bridgeAddress)
	if err != nil {
		return nil, fmt.Errorf("error resolving bridgeAddress %q: %w", bridgeAddress, err)
	}

	log.Println("Opening UDP connection to", addr)
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("error opening connection: %w", err)
	}

	return &bpupClient{
		conn: conn,
	}, nil
}

// StartListening blocks on the initial handshake with the server
// (described at http://docs-local.appbond.com/#section/Bond-Push-UDP-Protocol-(BPUP))
// and sets up a goroutine to send regular keep-alive signals to the bridge
func (c *bpupClient) StartListening(ctx context.Context) error {
	_, err := c.conn.Write([]byte("\n"))
	if err != nil {
		return fmt.Errorf("error sending initial message to server: %w", err)
	}

	buf := make([]byte, 256)
	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, err := c.conn.ReadFrom(buf)
	if err != nil {
		return fmt.Errorf("error reading handshake response from server: %w", err)
	}
	log.Println("Received handshake response from server:", string(buf[:n]))

	go func() {
		for {
			select {
			case <-time.After(60 * time.Second):
				sendKeepAlive(ctx, c.conn, 1*time.Second, 0)
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func sendKeepAlive(ctx context.Context, conn *net.UDPConn, backoff time.Duration, elapsed time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			if (elapsed + backoff) <= 120*time.Second {
				log.Printf("Retrying failed keep-alive after %s; failure was: %v\n", backoff, r)
				select {
				case <-time.After(backoff):
					sendKeepAlive(ctx, conn, 2*backoff, elapsed+backoff)
				case <-ctx.Done():
					return
				}
			} else {
				log.Printf("Not retrying failed keep-alive since %s have elapsed", elapsed)
				panic(r)
			}
		}
	}()
	_, err := conn.Write([]byte("\n"))
	if err != nil {
		panic(err)
	}
}
