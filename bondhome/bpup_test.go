package bondhome

import (
	"context"
	"net"
	"testing"
	"time"
)

type testServer interface {
	Address() string
	Stop() error
}

type udpTestServer struct {
	cancel context.CancelFunc
	conn   *net.UDPConn
}

func (s *udpTestServer) Address() string {
	return s.conn.LocalAddr().String()
}

func (s *udpTestServer) Stop() error {
	s.cancel()
	return s.conn.Close()
}

func startTestServer(ctx context.Context, t *testing.T, messageHandler func(message string) *string) testServer {
	t.Helper()
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		t.Fatalf("Error starting test UDP server: %v", err)
	}

	t.Logf("Started listening on %q", conn.LocalAddr().String())

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		for {
			select {
			case <-ctx.Done():
				t.Log("Terminating server loop")
				return
			default:
				err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
				if err != nil {
					t.Fatalf("Got error setting connection read deadline: %v", err)
				}

				buffer := make([]byte, 100)
				n, fromAddr, err := conn.ReadFromUDP(buffer)
				if err != nil {
					if e, ok := err.(net.Error); ok && e.Timeout() {
						continue
					} else if ctx.Err() == nil {
						t.Logf("Error while listening on %s: %v", conn.LocalAddr(), err)
					}
				}
				if n > 0 {
					t.Logf("Got %d bytes from %s containing: %q", n, fromAddr, buffer[:n])
					response := messageHandler(string(buffer[:n]))
					if response != nil {
						_, err := conn.WriteToUDP([]byte(*response), fromAddr)
						if err != nil {
							t.Fatalf("Error writing server response %q: %v", *response, err)
						}
					}
				}
			}
		}
	}()

	return &udpTestServer{
		cancel: cancel,
		conn:   conn,
	}
}

func Test_udpTestServer(t *testing.T) {
	serverResponse := "hi"
	ctx := context.Background()
	srv := startTestServer(ctx, t, func(_ string) *string {
		return &serverResponse
	})
	defer srv.Stop()

	addr, err := net.ResolveUDPAddr("udp", srv.Address())
	if err != nil {
		t.Fatalf("Error resolving address of test server: %v", err)
	}

	t.Logf("Resolved server address to %q", addr.String())

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatalf("Error opening local connection: %v", err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte("hello"))
	if err != nil {
		t.Errorf("Error sending message: %v", err)
	}
	_, err = conn.Write([]byte("it's"))
	if err != nil {
		t.Errorf("Error sending message: %v", err)
	}
	_, err = conn.Write([]byte("me"))
	if err != nil {
		t.Errorf("Error sending message: %v", err)
	}

	for i := 0; i < 3; i++ {
		buf := make([]byte, 100)
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			t.Fatalf("Got error reading response from server: %v", err)
		}
		actualResponse := string(buf[:n])
		t.Logf("Got response from server: %q", actualResponse)
		if actualResponse != serverResponse {
			t.Fatalf("Expected %q but got %q", serverResponse, actualResponse)
		}
	}
}

func Test_StartListening(t *testing.T) {
	initialResponse := `{"B":"ZZBL12345"}\n`
	ctx := context.Background()

	received := make(chan string, 3)

	// Set up a channel to facilitate the initial server handshake
	firstReceive := make(chan struct{}, 1)
	firstReceive <- struct{}{}

	srv := startTestServer(ctx, t, func(msg string) *string {
		received <- msg
		if _, ok := <-firstReceive; ok { // this is the first receive
			close(firstReceive)
			return &initialResponse
		}
		return nil
	})
	defer srv.Stop()

	c, err := NewClient(srv.Address())
	if err != nil {
		t.Fatalf("Error creating client: %v", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err = c.StartListening(ctx)
	if err != nil {
		t.Fatalf("Error calling StartListening: %v", err)
	}

	select {
	case msg := <-received:
		if msg != "\n" {
			t.Fatalf("Expected initial message to be '\\n' but was %q", msg)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("Server didn't receive initial message within 3 seconds")
	}

	select {
	case msg := <-received:
		if msg != "\n" {
			t.Fatalf("Expected keepalive message to be '\\n' but was %q", msg)
		}
	case <-time.After(90 * time.Second):
		t.Fatalf("Expected keepalive signal within 90 seconds")
	}
}
