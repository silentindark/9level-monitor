package store

import (
	"math"
	"sync"
	"time"
)

// LatencyStats tracks response times for external systems
type LatencyStats struct {
	AMIMS       float64   `json:"ami_ms"`
	ARIMS       float64   `json:"ari_ms"`
	RTPPollMS   float64   `json:"rtp_poll_ms"`
	EventsPerSec float64  `json:"events_per_sec"`
	AMIQueueLen int       `json:"ami_queue_len"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Store holds all monitoring state in memory
type Store struct {
	mu sync.RWMutex

	Channels  map[string]*ChannelState  // keyed by channel name
	Endpoints map[string]*EndpointState // keyed by endpoint name
	Bridges   map[string][]string       // bridge ID → channel names

	PeakCalls int
	StartTime time.Time

	SecurityEvents []*SecurityEvent
	securitySeq    int
	securityByIP   map[string]*ipTracker

	Latency     LatencyStats
	eventCount  int64
	lastEventTs time.Time
}

func New() *Store {
	return &Store{
		Channels:       make(map[string]*ChannelState),
		Endpoints:      make(map[string]*EndpointState),
		Bridges:        make(map[string][]string),
		SecurityEvents: make([]*SecurityEvent, 0),
		securityByIP:   make(map[string]*ipTracker),
		StartTime:      time.Now(),
	}
}

// --- Channel operations ---

func (s *Store) SetChannel(name string, ch *ChannelState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Channels[name] = ch
	if len(s.Channels) > s.PeakCalls {
		s.PeakCalls = len(s.Channels)
	}
}

func (s *Store) GetChannel(name string) *ChannelState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Channels[name]
}

func (s *Store) RemoveChannel(name string) *ChannelState {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := s.Channels[name]
	delete(s.Channels, name)
	return ch
}

func (s *Store) GetAllChannels() []*ChannelState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	result := make([]*ChannelState, 0, len(s.Channels))
	for _, ch := range s.Channels {
		ch.Duration = int(now.Sub(ch.CreationTime).Seconds())
		result = append(result, ch)
	}
	return result
}

func (s *Store) ChannelNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.Channels))
	for name := range s.Channels {
		names = append(names, name)
	}
	return names
}

// --- Endpoint operations ---

func (s *Store) SetEndpoint(name string, ep *EndpointState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Endpoints[name] = ep
}

func (s *Store) GetEndpoint(name string) *EndpointState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Endpoints[name]
}

func (s *Store) GetOrCreateEndpoint(name string) *EndpointState {
	s.mu.Lock()
	defer s.mu.Unlock()
	ep, ok := s.Endpoints[name]
	if !ok {
		ep = &EndpointState{
			Name:     name,
			State:    "OFFLINE",
			Contacts: make(map[string]*ContactState),
		}
		s.Endpoints[name] = ep
	}
	return ep
}

func (s *Store) GetAllEndpoints() []*EndpointState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*EndpointState, 0, len(s.Endpoints))
	for _, ep := range s.Endpoints {
		result = append(result, ep)
	}
	return result
}

// --- Bridge operations ---

func (s *Store) AddToBridge(bridgeID, channelName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Bridges[bridgeID] = append(s.Bridges[bridgeID], channelName)
}

func (s *Store) RemoveFromBridge(bridgeID, channelName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	channels := s.Bridges[bridgeID]
	for i, ch := range channels {
		if ch == channelName {
			s.Bridges[bridgeID] = append(channels[:i], channels[i+1:]...)
			break
		}
	}
	if len(s.Bridges[bridgeID]) == 0 {
		delete(s.Bridges, bridgeID)
	}
}

// --- Summary ---

type Summary struct {
	ActiveCalls         int     `json:"active_calls"`
	RegisteredEndpoints int     `json:"registered_endpoints"`
	TotalEndpoints      int     `json:"total_endpoints"`
	AvgRxMES            float64 `json:"avg_rxmes"`
	AvgTxMES            float64 `json:"avg_txmes"`
	PeakCalls           int     `json:"peak_calls"`
	UptimeSeconds       int     `json:"uptime_seconds"`
}

func (s *Store) GetSummary() Summary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalRxMES, totalTxMES float64
	var mesCount int

	for _, ch := range s.Channels {
		if ch.Quality != nil && ch.Quality.RxMES > 0 {
			totalRxMES += ch.Quality.RxMES
			totalTxMES += ch.Quality.TxMES
			mesCount++
		}
	}

	var avgRx, avgTx float64
	if mesCount > 0 {
		avgRx = math.Round(totalRxMES/float64(mesCount)*100) / 100
		avgTx = math.Round(totalTxMES/float64(mesCount)*100) / 100
	}

	registered := 0
	for _, ep := range s.Endpoints {
		if ep.State == "ONLINE" {
			registered++
		}
	}

	return Summary{
		ActiveCalls:         len(s.Channels),
		RegisteredEndpoints: registered,
		TotalEndpoints:      len(s.Endpoints),
		AvgRxMES:            avgRx,
		AvgTxMES:            avgTx,
		PeakCalls:           s.PeakCalls,
		UptimeSeconds:       int(time.Since(s.StartTime).Seconds()),
	}
}

// ClearEndpoints removes all endpoints (for re-bootstrap)
func (s *Store) ClearEndpoints() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Endpoints = make(map[string]*EndpointState)
}

// UpdateLatency updates latency stats for a given source
func (s *Store) UpdateLatency(amiMS, ariMS, rtpPollMS float64, amiQueueLen int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.eventCount++

	if !s.lastEventTs.IsZero() {
		elapsed := now.Sub(s.lastEventTs).Seconds()
		if elapsed > 0 {
			// Exponential moving average for events/sec
			instantRate := 1.0 / elapsed
			if s.Latency.EventsPerSec == 0 {
				s.Latency.EventsPerSec = instantRate
			} else {
				s.Latency.EventsPerSec = s.Latency.EventsPerSec*0.9 + instantRate*0.1
			}
		}
	}
	s.lastEventTs = now

	if amiMS >= 0 {
		s.Latency.AMIMS = amiMS
	}
	if ariMS >= 0 {
		s.Latency.ARIMS = ariMS
	}
	if rtpPollMS >= 0 {
		s.Latency.RTPPollMS = rtpPollMS
	}
	if amiQueueLen >= 0 {
		s.Latency.AMIQueueLen = amiQueueLen
	}
	s.Latency.UpdatedAt = now
}

// GetLatency returns current latency stats
func (s *Store) GetLatency() LatencyStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Latency
}
