package bondhome

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/golang/glog"
)

// Update represents an update message from the Bond Bridge
type Update struct {
	BondID     string          `json:"B"`
	Topic      string          `json:"t"`
	StatusCode int             `json:"s"`
	HTTPMethod uint8           `json:"m"`
	Body       json.RawMessage `json:"b"`

	// Error fields
	ErrorID  int    `json:"err_id"`
	ErrorMsg string `json:"err_msg"`
}

// Timeout is returned when an operation times out
type Timeout error

// PushClient is an interface for receiving messages
// pushed from a Bond Home bridge
type PushClient interface {
	// StartListening initiates a connection to listen for
	// updates from the Bond Home bridge.
	// If no error is returned, it is the caller's responsibility
	// to call StopListening to release any resources used by the PushClient
	StartListening() error

	// StopListening closes out any resources used by the PushClient
	StopListening() error

	// Receive waits for an update from the server, up to
	// a specified timeout. If the receive times out,
	// the returned error will be of type Timeout.
	Receive(timeout time.Duration) (*Update, error)
}

type bpupClient struct {
	ctx    context.Context
	cancel context.CancelFunc
	conn   *net.UDPConn
}

// NewClient creates a new PushClient that receives updates
// from the bridge at the given address
func NewClient(ctx context.Context, bridgeAddress string) (PushClient, error) {
	addr, err := net.ResolveUDPAddr("udp", bridgeAddress)
	if err != nil {
		return nil, fmt.Errorf("error resolving bridgeAddress %q: %w", bridgeAddress, err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("error opening connection: %w", err)
	}

	glog.Infoln("Opened UDP connection to", addr, "listening at", conn.LocalAddr())
	ctx, cancel := context.WithCancel(ctx)

	return &bpupClient{ctx, cancel, conn}, nil
}

// StartListening blocks on the initial handshake with the server
// (described at http://docs-local.appbond.com/#section/Bond-Push-UDP-Protocol-(BPUP))
// and sets up a goroutine to send regular keep-alive signals to the bridge
func (c *bpupClient) StartListening() error {
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
	glog.Infoln("Received handshake response from server:", string(buf[:n]))

	go func() {
		for {
			select {
			case <-time.After(60 * time.Second):
				ctx, cancel := context.WithTimeout(c.ctx, 120*time.Second)
				sendKeepAlive(ctx, c.conn, 1*time.Second, 0)
				cancel()
			case <-c.ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (c *bpupClient) StopListening() error {
	defer c.cancel()
	err := c.conn.Close()
	if err != nil {
		return fmt.Errorf("error closing connection: %w", err)
	}
	return nil
}

func (c *bpupClient) Receive(timeout time.Duration) (*Update, error) {
	c.conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 512) // 512B message buffer
	n, err := c.conn.Read(buf)
	if err != nil {
		if e, ok := err.(net.Error); ok && e.Timeout() {
			return nil, Timeout(e)
		}
		return nil, err
	}
	glog.V(1).Infof("Received UDP message from server: %q", string(buf[:n]))
	trimmed := strings.TrimSpace(string(buf[:n]))
	update := &Update{}
	err = json.Unmarshal([]byte(trimmed), update)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling %q: %w", trimmed, err)
	}
	return update, nil
}

func sendKeepAlive(ctx context.Context, conn *net.UDPConn, backoff time.Duration, elapsed time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			glog.Warningf("Retrying failed keep-alive after %s; failure was: %v\n", backoff, r)
			select {
			case <-time.After(backoff):
				sendKeepAlive(ctx, conn, 2*backoff, elapsed+backoff)
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					glog.Errorf("Not retrying failed keep-alive since %s have elapsed", elapsed)
					panic(r)
				}
				glog.Warning("Canceling keep-alive retry loop")
				return
			}
		}
	}()
	_, err := conn.Write([]byte("\n"))
	if err != nil {
		panic(err)
	}
}
