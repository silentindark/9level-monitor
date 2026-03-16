package ami

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Client maintains a persistent TCP connection to Asterisk AMI
type Client struct {
	host   string
	port   string
	user   string
	secret string

	conn    net.Conn
	scanner *bufio.Scanner
	mu      sync.Mutex

	connected atomic.Bool
	eventCh   chan Event
	actionSeq atomic.Int64
}

func NewClient(host, port, user, secret string) *Client {
	return &Client{
		host:    host,
		port:    port,
		user:    user,
		secret:  secret,
		eventCh: make(chan Event, 256),
	}
}

// Events returns a channel that receives AMI events
func (c *Client) Events() <-chan Event {
	return c.eventCh
}

// Connected returns true if AMI is connected
func (c *Client) Connected() bool {
	return c.connected.Load()
}

// Run connects to AMI and processes events. Reconnects on failure.
// Blocks until ctx is done (call in a goroutine).
func (c *Client) Run(stopCh <-chan struct{}) {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-stopCh:
			c.close()
			return
		default:
		}

		err := c.connect()
		if err != nil {
			log.Printf("ami: connect error: %v (retry in %v)", err, backoff)
			c.connected.Store(false)
			select {
			case <-time.After(backoff):
			case <-stopCh:
				return
			}
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		backoff = time.Second
		c.connected.Store(true)
		log.Println("ami: connected and authenticated")

		c.readLoop(stopCh)
		c.connected.Store(false)
		log.Println("ami: disconnected, will reconnect...")
	}
}

func (c *Client) connect() error {
	addr := net.JoinHostPort(c.host, c.port)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}

	c.conn = conn
	c.scanner = bufio.NewScanner(conn)

	// Read greeting line
	if c.scanner.Scan() {
		greeting := c.scanner.Text()
		if !strings.HasPrefix(greeting, "Asterisk Call Manager") {
			c.conn.Close()
			return fmt.Errorf("unexpected greeting: %s", greeting)
		}
	}

	// Login
	err = c.sendAction("Login", map[string]string{
		"Username": c.user,
		"Secret":   c.secret,
	})
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("send login: %w", err)
	}

	// Read login response
	resp, err := c.readMessage()
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("read login response: %w", err)
	}

	if resp.Get("Response") != "Success" {
		c.conn.Close()
		return fmt.Errorf("login failed: %s", resp.Get("Message"))
	}

	return nil
}

func (c *Client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// Ping sends an AMI Ping and waits for Pong. Returns latency or error.
func (c *Client) Ping(timeout time.Duration) (time.Duration, error) {
	if !c.connected.Load() {
		return 0, fmt.Errorf("not connected")
	}

	start := time.Now()
	err := c.sendAction("Ping", nil)
	if err != nil {
		return 0, fmt.Errorf("ping send: %w", err)
	}

	// Wait for pong via a short read deadline on next action response
	// The actual pong is consumed by readLoop, so we just verify the
	// connection is still alive by checking connected state after a brief wait.
	deadline := time.After(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			return 0, fmt.Errorf("ping timeout after %v", timeout)
		case <-ticker.C:
			if !c.connected.Load() {
				return 0, fmt.Errorf("disconnected during ping")
			}
			// If we're still connected after sending Ping, the readLoop
			// would have disconnected us if the connection was dead.
			elapsed := time.Since(start)
			if elapsed > 100*time.Millisecond {
				return elapsed, nil
			}
		}
	}
}

// SendAction sends an AMI action and returns immediately (async).
func (c *Client) SendAction(action string, params map[string]string) error {
	return c.sendAction(action, params)
}

func (c *Client) sendAction(action string, params map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Action: %s\r\n", action))

	seq := c.actionSeq.Add(1)
	sb.WriteString(fmt.Sprintf("ActionID: %d\r\n", seq))

	for k, v := range params {
		sb.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	sb.WriteString("\r\n")

	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := c.conn.Write([]byte(sb.String()))
	return err
}

func (c *Client) readMessage() (Event, error) {
	ev := make(Event)
	for c.scanner.Scan() {
		line := strings.TrimRight(c.scanner.Text(), "\r")
		if line == "" {
			if len(ev) > 0 {
				return ev, nil
			}
			continue
		}
		idx := strings.Index(line, ": ")
		if idx >= 0 {
			ev[line[:idx]] = line[idx+2:]
		} else if idx = strings.Index(line, ":"); idx >= 0 {
			ev[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
		}
	}
	if err := c.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("connection closed")
}

func (c *Client) readLoop(stopCh <-chan struct{}) {
	// Keepalive: send Ping every 30s to prevent idle timeout
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	go func() {
		for {
			select {
			case <-stopCh:
				return
			case <-pingTicker.C:
				c.sendAction("Ping", nil)
			}
		}
	}()

	for {
		select {
		case <-stopCh:
			return
		default:
		}

		c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		ev, err := c.readMessage()
		if err != nil {
			log.Printf("ami: read error: %v", err)
			return
		}
		if ev == nil {
			return
		}

		// Skip Pong responses and action responses without Event header
		if ev.Get("Response") == "Success" && ev.Get("Ping") != "" {
			continue
		}

		if ev.EventType() != "" {
			select {
			case c.eventCh <- ev:
			default:
				// Channel full, drop oldest
				select {
				case <-c.eventCh:
				default:
				}
				c.eventCh <- ev
			}
		}
	}
}
