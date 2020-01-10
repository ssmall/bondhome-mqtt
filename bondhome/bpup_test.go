package bondhome

import (
	"context"
	"net"
	"testing"
	"time"
)

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

func (s *udpTestServer) Send(t *testing.T, msg string, c *bpupClient) {
	t.Helper()
	a, ok := c.conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("Cannot cast %v to *net.UDPAddr", c.conn.LocalAddr())
	}
	t.Logf("Sending message %q to client @ %s", msg, a)
	_, err := s.conn.WriteToUDP([]byte(msg), a)
	if err != nil {
		t.Fatal("Erroring sending message:", err)
	}
}

func startTestServer(ctx context.Context, t *testing.T, messageHandler func(message string) *string) *udpTestServer {
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

func startTestServerWithHandshake(ctx context.Context, t *testing.T, messageHandler func(message string) *string) *udpTestServer {
	initialResponse := `{"B":"ZZBL12345"}\n`

	// Set up a channel to facilitate the initial server handshake
	firstReceive := make(chan struct{}, 1)
	firstReceive <- struct{}{}

	return startTestServer(ctx, t, func(message string) *string {
		resp := messageHandler(message)
		if _, ok := <-firstReceive; ok { // this is the first receive
			close(firstReceive)
			return &initialResponse
		}
		return resp
	})
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

// takes up to 90s to run
func Test_StartListening(t *testing.T) {
	ctx := context.Background()

	received := make(chan string, 3)

	srv := startTestServerWithHandshake(ctx, t, func(msg string) *string {
		received <- msg
		return nil
	})
	defer srv.Stop()

	c, err := NewClient(ctx, srv.Address())
	if err != nil {
		t.Fatalf("Error creating client: %v", err)
	}

	err = c.StartListening()
	if err != nil {
		t.Fatalf("Error calling StartListening: %v", err)
	}
	defer c.StopListening()

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

// takes at least 90s to run
func Test_StartListening_keepAliveError(t *testing.T) {
	t.Skip("Un-skip this test and look at the logs to see the keep-alive retry mechanism working")
	ctx := context.Background()
	srv := startTestServerWithHandshake(ctx, t, func(msg string) *string {
		return nil
	})
	defer srv.Stop()

	c, err := NewClient(ctx, srv.Address())
	if err != nil {
		t.Fatal("Error creating client:", err)
	}

	err = c.StartListening()
	if err != nil {
		t.Fatal("Error calling StartListening:", err)
	}
	defer c.StopListening()

	// Deliberately break the keep-alive functionality by forcibly
	// closing the client's UDP socket
	b := c.(*bpupClient)
	err = b.conn.Close()
	if err != nil {
		t.Fatal("Couldn't close connection:", err)
	}

	// Wait long enough for some retries to trigger but not long enough
	// for it to totally time out and panic
	time.Sleep(90 * time.Second)
}

func Test_Receive(t *testing.T) {
	updateMsg := `{"B":"ZZBL12345","t":"devices/aabbccdd/state","i":"00112233bbeeeeff","s":200,"m":0,"f":255,"b":{"_":"ab9284ef","power":1,"speed":2}}\n`

	expectedUpdate := Update{
		BondID:     "ZZBL12345",
		Topic:      "devices/aabbccdd/state",
		StatusCode: 200,
		HTTPMethod: uint8(0),
		Body: map[string]interface{}{
			"_":     "ab9284ef",
			"power": float64(1),
			"speed": float64(2),
		},
	}

	ctx := context.Background()

	srv := startTestServerWithHandshake(ctx, t, func(msg string) *string {
		return nil
	})
	defer srv.Stop()

	c, err := NewClient(ctx, srv.Address())
	if err != nil {
		t.Fatalf("Error creating client: %v", err)
	}

	err = c.StartListening()
	if err != nil {
		t.Fatalf("Error calling StartListening: %v", err)
	}
	defer c.StopListening()

	b, _ := c.(*bpupClient)

	srv.Send(t, updateMsg, b)
	update, err := c.Receive(1 * time.Second)
	if err != nil {
		t.Fatal("Error receiving update message:", err)
	}
	if update == nil {
		t.Fatalf("Actual is nil")
	}

	if !((update.BondID == expectedUpdate.BondID) &&
		(update.Topic == expectedUpdate.Topic) &&
		(update.StatusCode == expectedUpdate.StatusCode) &&
		(update.HTTPMethod == expectedUpdate.HTTPMethod) &&
		(update.Body != nil)) {
		t.Fatalf("Expected %#v but got %#v", expectedUpdate, *update)
	}
}

func sendMsgToClient(t *testing.T, c *bpupClient, msg string) {
	t.Helper()
	a, ok := c.conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("Cannot cast %v to *net.UDPAddr", c.conn.LocalAddr())
	}
	conn, err := net.DialUDP("udp", nil, a)
	if err != nil {
		t.Fatal("Error creating local UDP connection:", err)
	}
	_, err = conn.Write([]byte(msg))
	if err != nil {
		t.Fatal("Erroring sending message:", err)
	}
}
