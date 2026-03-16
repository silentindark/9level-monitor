package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// SSEEvent represents a typed SSE event
type SSEEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Broker manages Server-Sent Events clients with rate limiting for security events
type Broker struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}

	// Security event batching
	secMu       sync.Mutex
	secBuffer   []any
	secInterval time.Duration
}

func NewBroker() *Broker {
	b := &Broker{
		clients:     make(map[chan []byte]struct{}),
		secBuffer:   make([]any, 0, 64),
		secInterval: 1 * time.Second,
	}
	go b.securityFlusher()
	return b
}

// securityFlusher periodically flushes batched security events
func (b *Broker) securityFlusher() {
	ticker := time.NewTicker(b.secInterval)
	defer ticker.Stop()
	for range ticker.C {
		b.flushSecurityEvents()
	}
}

func (b *Broker) flushSecurityEvents() {
	b.secMu.Lock()
	if len(b.secBuffer) == 0 {
		b.secMu.Unlock()
		return
	}
	batch := b.secBuffer
	b.secBuffer = make([]any, 0, 64)
	b.secMu.Unlock()

	// Send as a single batch event
	b.broadcast("security:events", map[string]any{
		"events": batch,
		"count":  len(batch),
	})
}

func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan []byte, 32)

	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		delete(b.clients, ch)
		b.mu.Unlock()
		close(ch)
	}()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

// Send broadcasts a typed event to all SSE clients.
// Security events are batched to prevent browser flood during brute force attacks.
func (b *Broker) Send(eventType string, data any) {
	if eventType == "security:event" {
		b.secMu.Lock()
		b.secBuffer = append(b.secBuffer, data)
		b.secMu.Unlock()
		return
	}
	b.broadcast(eventType, data)
}

func (b *Broker) broadcast(eventType string, data any) {
	evt := SSEEvent{Type: eventType, Data: data}
	raw, err := json.Marshal(evt)
	if err != nil {
		log.Printf("sse: marshal error: %v", err)
		return
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.clients {
		select {
		case ch <- raw:
		default:
			// slow client, skip
		}
	}
}

// ClientCount returns connected client count
func (b *Broker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
