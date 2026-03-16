package store

import (
	"sort"
	"time"
)

const maxSecurityEvents = 2000

// SecurityEvent represents a single security incident from AMI
type SecurityEvent struct {
	ID            int       `json:"id"`
	EventType     string    `json:"event_type"`
	AccountID     string    `json:"account_id"`
	RemoteAddress string    `json:"remote_address"`
	Service       string    `json:"service"`
	Timestamp     time.Time `json:"timestamp"`
}

// SecuritySummary provides aggregated security stats
type SecuritySummary struct {
	TotalEvents  int            `json:"total_events"`
	EventsByType map[string]int `json:"events_by_type"`
	TopOffenders []OffenderStat `json:"top_offenders"`
}

// SecurityPage represents a paginated response
type SecurityPage struct {
	Events []*SecurityEvent `json:"events"`
	Total  int              `json:"total"`
	Page   int              `json:"page"`
	Pages  int              `json:"pages"`
}

// OffenderStat tracks per-IP attack counts
type OffenderStat struct {
	RemoteAddress string    `json:"remote_address"`
	Count         int       `json:"count"`
	LastSeen      time.Time `json:"last_seen"`
	LastAccount   string    `json:"last_account"`
	EventTypes    []string  `json:"event_types"`
}

type ipTracker struct {
	count      int
	lastSeen   time.Time
	lastAcct   string
	eventTypes map[string]struct{}
}

// AddSecurityEvent stores a security event (capped buffer)
func (s *Store) AddSecurityEvent(ev *SecurityEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.securitySeq++
	ev.ID = s.securitySeq

	if len(s.SecurityEvents) >= maxSecurityEvents {
		s.SecurityEvents = s.SecurityEvents[1:]
	}
	s.SecurityEvents = append(s.SecurityEvents, ev)

	ip := ev.RemoteAddress
	t, ok := s.securityByIP[ip]
	if !ok {
		t = &ipTracker{eventTypes: make(map[string]struct{})}
		s.securityByIP[ip] = t
	}
	t.count++
	t.lastSeen = ev.Timestamp
	t.lastAcct = ev.AccountID
	t.eventTypes[ev.EventType] = struct{}{}
}

// GetSecurityEventsPage returns a paginated view (newest first)
func (s *Store) GetSecurityEventsPage(page, perPage int) SecurityPage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n := len(s.SecurityEvents)
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}

	pages := (n + perPage - 1) / perPage
	if pages < 1 {
		pages = 1
	}
	if page > pages {
		page = pages
	}

	// Events are stored oldest-first, we want newest-first
	start := n - page*perPage
	end := n - (page-1)*perPage
	if start < 0 {
		start = 0
	}

	result := make([]*SecurityEvent, 0, end-start)
	for i := end - 1; i >= start; i-- {
		result = append(result, s.SecurityEvents[i])
	}

	return SecurityPage{
		Events: result,
		Total:  n,
		Page:   page,
		Pages:  pages,
	}
}

// GetSecuritySummary returns aggregated security stats
func (s *Store) GetSecuritySummary() SecuritySummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	byType := make(map[string]int)
	for _, ev := range s.SecurityEvents {
		byType[ev.EventType]++
	}

	offenders := make([]OffenderStat, 0, len(s.securityByIP))
	for ip, t := range s.securityByIP {
		types := make([]string, 0, len(t.eventTypes))
		for et := range t.eventTypes {
			types = append(types, et)
		}
		offenders = append(offenders, OffenderStat{
			RemoteAddress: ip,
			Count:         t.count,
			LastSeen:      t.lastSeen,
			LastAccount:   t.lastAcct,
			EventTypes:    types,
		})
	}
	sort.Slice(offenders, func(i, j int) bool {
		return offenders[i].Count > offenders[j].Count
	})
	if len(offenders) > 10 {
		offenders = offenders[:10]
	}

	return SecuritySummary{
		TotalEvents:  len(s.SecurityEvents),
		EventsByType: byType,
		TopOffenders: offenders,
	}
}

// PurgeSecurityEvents removes events older than the given duration
func (s *Store) PurgeSecurityEvents(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	kept := 0
	for _, ev := range s.SecurityEvents {
		if ev.Timestamp.After(cutoff) {
			s.SecurityEvents[kept] = ev
			kept++
		}
	}
	purged := len(s.SecurityEvents) - kept
	s.SecurityEvents = s.SecurityEvents[:kept]

	// Rebuild IP tracker from remaining events
	s.securityByIP = make(map[string]*ipTracker)
	for _, ev := range s.SecurityEvents {
		ip := ev.RemoteAddress
		t, ok := s.securityByIP[ip]
		if !ok {
			t = &ipTracker{eventTypes: make(map[string]struct{})}
			s.securityByIP[ip] = t
		}
		t.count++
		t.lastSeen = ev.Timestamp
		t.lastAcct = ev.AccountID
		t.eventTypes[ev.EventType] = struct{}{}
	}

	return purged
}
