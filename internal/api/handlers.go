package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/9level/9level-monitor/internal/alerts"
	"github.com/9level/9level-monitor/internal/db"
	"github.com/9level/9level-monitor/internal/store"
)

type Handler struct {
	store        *store.Store
	broker       *Broker
	db           *db.DB
	amiConnected func() bool
	ariHealthy   func() bool
	alertEngine  *alerts.Engine
}

func NewHandler(s *store.Store, b *Broker, database *db.DB,
	amiConn func() bool, ariHealth func() bool, alertEng *alerts.Engine) *Handler {
	return &Handler{
		store:        s,
		broker:       b,
		db:           database,
		amiConnected: amiConn,
		ariHealthy:   ariHealth,
		alertEngine:  alertEng,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/monitor", h.getMonitor)
	mux.HandleFunc("GET /api/v1/calls", h.getCalls)
	mux.HandleFunc("GET /api/v1/calls/{id}", h.getCall)
	mux.HandleFunc("GET /api/v1/endpoints", h.getEndpoints)
	mux.HandleFunc("GET /api/v1/summary", h.getSummary)
	mux.HandleFunc("GET /api/v1/health", h.getHealth)
	mux.Handle("GET /api/v1/events", h.broker)

	mux.HandleFunc("GET /api/v1/security", h.getSecurityEvents)
	mux.HandleFunc("GET /api/v1/security/summary", h.getSecuritySummary)

	// History routes (SQLite)
	mux.HandleFunc("GET /api/v1/history/calls", h.getHistoryCalls)
	mux.HandleFunc("GET /api/v1/history/calls/stats", h.getHistoryCallStats)
	mux.HandleFunc("GET /api/v1/history/calls/hourly", h.getHistoryCallsHourly)
	mux.HandleFunc("GET /api/v1/history/calls/daily", h.getHistoryCallsDaily)
	mux.HandleFunc("GET /api/v1/history/security", h.getHistorySecurity)
	mux.HandleFunc("GET /api/v1/history/endpoints", h.getHistoryEndpoints)
	mux.HandleFunc("GET /api/v1/db/size", h.getDBSize)

	// Admin routes
	mux.HandleFunc("GET /api/v1/admin/settings", h.getSettings)
	mux.HandleFunc("PUT /api/v1/admin/settings", h.putSettings)
	mux.HandleFunc("POST /api/v1/admin/test/telegram", h.testTelegram)
	mux.HandleFunc("POST /api/v1/admin/test/webhook", h.testWebhook)

	// Serve frontend
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "/frontend/index.html")
	})

	// Backward compat with v1 frontend
	mux.HandleFunc("GET /api/monitor", h.getMonitor)
	mux.HandleFunc("GET /api/calls", h.getCalls)
	mux.HandleFunc("GET /api/endpoints", h.getEndpoints)
	mux.HandleFunc("GET /api/summary", h.getSummary)
	mux.HandleFunc("GET /api/health", h.getHealth)
	mux.HandleFunc("GET /api/security", h.getSecurityEvents)
	mux.HandleFunc("GET /api/security/summary", h.getSecuritySummary)
	mux.Handle("GET /api/events", h.broker)
}

func (h *Handler) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) getMonitor(w http.ResponseWriter, r *http.Request) {
	calls := h.store.GetAllChannels()
	endpoints := h.store.GetAllEndpoints()
	summary := h.store.GetSummary()

	// Convert endpoint contacts map to slice for JSON compat
	type epJSON struct {
		Name     string              `json:"name"`
		State    string              `json:"state"`
		Contacts []*store.ContactState `json:"contacts"`
	}
	epList := make([]epJSON, 0, len(endpoints))
	for _, ep := range endpoints {
		contacts := make([]*store.ContactState, 0, len(ep.Contacts))
		for _, c := range ep.Contacts {
			contacts = append(contacts, c)
		}
		epList = append(epList, epJSON{Name: ep.Name, State: ep.State, Contacts: contacts})
	}

	h.writeJSON(w, map[string]any{
		"calls":     calls,
		"endpoints": epList,
		"summary":   summary,
	})
}

func (h *Handler) getCalls(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, h.store.GetAllChannels())
}

func (h *Handler) getCall(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Try by channel name (may contain slashes, so also check all channels)
	ch := h.store.GetChannel(id)
	if ch == nil {
		// Search by uniqueid
		for _, c := range h.store.GetAllChannels() {
			if c.UniqueID == id || strings.Contains(c.Name, id) {
				ch = c
				break
			}
		}
	}
	if ch == nil {
		http.Error(w, `{"error":"channel not found"}`, http.StatusNotFound)
		return
	}
	h.writeJSON(w, ch)
}

func (h *Handler) getEndpoints(w http.ResponseWriter, r *http.Request) {
	endpoints := h.store.GetAllEndpoints()
	type epJSON struct {
		Name     string              `json:"name"`
		State    string              `json:"state"`
		Contacts []*store.ContactState `json:"contacts"`
	}
	result := make([]epJSON, 0, len(endpoints))
	for _, ep := range endpoints {
		contacts := make([]*store.ContactState, 0, len(ep.Contacts))
		for _, c := range ep.Contacts {
			contacts = append(contacts, c)
		}
		result = append(result, epJSON{Name: ep.Name, State: ep.State, Contacts: contacts})
	}
	h.writeJSON(w, result)
}

func (h *Handler) getSummary(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, h.store.GetSummary())
}

func (h *Handler) getSecurityEvents(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	h.writeJSON(w, h.store.GetSecurityEventsPage(page, perPage))
}

func (h *Handler) getSecuritySummary(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, h.store.GetSecuritySummary())
}

func (h *Handler) getHealth(w http.ResponseWriter, r *http.Request) {
	lat := h.store.GetLatency()

	// AMI health: connected + receiving events recently (< 60s)
	amiStatus := "disconnected"
	if h.amiConnected() {
		if !lat.UpdatedAt.IsZero() && time.Since(lat.UpdatedAt) < 60*time.Second {
			amiStatus = "connected"
		} else {
			amiStatus = "stale"
		}
	}

	h.writeJSON(w, map[string]any{
		"ami":            amiStatus,
		"ari":            boolToStatus(h.ariHealthy()),
		"channels":       len(h.store.GetAllChannels()),
		"endpoints":      len(h.store.GetAllEndpoints()),
		"sse":            h.broker.ClientCount(),
		"ami_ms":         lat.AMIMS,
		"ari_ms":         lat.ARIMS,
		"rtp_poll_ms":    lat.RTPPollMS,
		"events_per_sec": lat.EventsPerSec,
		"ami_queue_len":  lat.AMIQueueLen,
	})
}

// --- History API (SQLite) ---

func (h *Handler) parseTimeRange(r *http.Request) (time.Time, time.Time) {
	q := r.URL.Query()
	now := time.Now()
	from := now.AddDate(0, 0, -1) // default: last 24h
	to := now

	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			from = t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			to = t.AddDate(0, 0, 1) // end of day
		}
	}
	return from, to
}

func (h *Handler) getHistoryCalls(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		http.Error(w, `{"error":"database not available"}`, http.StatusServiceUnavailable)
		return
	}
	from, to := h.parseTimeRange(r)
	q := r.URL.Query()
	minMOS, _ := strconv.ParseFloat(q.Get("min_mos"), 64)
	page, _ := strconv.Atoi(q.Get("page"))
	perPage, _ := strconv.Atoi(q.Get("per_page"))

	result, err := h.db.QueryCalls(from, to, minMOS, page, perPage)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	h.writeJSON(w, result)
}

func (h *Handler) getHistoryCallStats(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		http.Error(w, `{"error":"database not available"}`, http.StatusServiceUnavailable)
		return
	}
	from, to := h.parseTimeRange(r)

	result, err := h.db.QueryCallStats(from, to)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	h.writeJSON(w, result)
}

func (h *Handler) getHistoryCallsHourly(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		http.Error(w, `{"error":"database not available"}`, http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	from := today
	to := today.AddDate(0, 0, 1)

	// Support legacy "date" param
	if v := q.Get("date"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = t
			to = t.AddDate(0, 0, 1)
		}
	}
	// from/to override date
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			to = t.AddDate(0, 0, 1)
		}
	}

	result, err := h.db.QueryCallsHourly(from, to)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	h.writeJSON(w, result)
}

func (h *Handler) getHistoryCallsDaily(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		http.Error(w, `{"error":"database not available"}`, http.StatusServiceUnavailable)
		return
	}
	from, to := h.parseTimeRange(r)
	result, err := h.db.QueryCallsDaily(from, to)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	h.writeJSON(w, result)
}

func (h *Handler) getHistorySecurity(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		http.Error(w, `{"error":"database not available"}`, http.StatusServiceUnavailable)
		return
	}
	from, to := h.parseTimeRange(r)
	q := r.URL.Query()
	eventType := q.Get("type")
	page, _ := strconv.Atoi(q.Get("page"))
	perPage, _ := strconv.Atoi(q.Get("per_page"))

	result, err := h.db.QuerySecurityEvents(from, to, eventType, page, perPage)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	h.writeJSON(w, result)
}

func (h *Handler) getHistoryEndpoints(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		http.Error(w, `{"error":"database not available"}`, http.StatusServiceUnavailable)
		return
	}
	from, to := h.parseTimeRange(r)
	q := r.URL.Query()
	endpoint := q.Get("endpoint")
	page, _ := strconv.Atoi(q.Get("page"))
	perPage, _ := strconv.Atoi(q.Get("per_page"))

	result, err := h.db.QueryEndpointChanges(from, to, endpoint, page, perPage)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	h.writeJSON(w, result)
}

func (h *Handler) getDBSize(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		h.writeJSON(w, map[string]any{"size_bytes": 0, "size_human": "N/A"})
		return
	}
	size, err := h.db.Size()
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	h.writeJSON(w, map[string]any{
		"size_bytes": size,
		"size_human": humanSize(size),
	})
}

func humanSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return strconv.FormatFloat(float64(bytes)/float64(GB), 'f', 1, 64) + " GB"
	case bytes >= MB:
		return strconv.FormatFloat(float64(bytes)/float64(MB), 'f', 1, 64) + " MB"
	case bytes >= KB:
		return strconv.FormatFloat(float64(bytes)/float64(KB), 'f', 1, 64) + " KB"
	default:
		return strconv.FormatInt(bytes, 10) + " B"
	}
}

func boolToStatus(b bool) string {
	if b {
		return "connected"
	}
	return "disconnected"
}

// --- Admin API ---

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		http.Error(w, `{"error":"database not available"}`, http.StatusServiceUnavailable)
		return
	}
	settings, err := h.db.GetAllSettings()
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	h.writeJSON(w, settings)
}

func (h *Handler) putSettings(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		http.Error(w, `{"error":"database not available"}`, http.StatusServiceUnavailable)
		return
	}
	var settings map[string]string
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}
	if err := h.db.SetSettings(settings); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if h.alertEngine != nil {
		h.alertEngine.Reload()
	}
	h.writeJSON(w, map[string]bool{"ok": true})
}

func (h *Handler) testTelegram(w http.ResponseWriter, r *http.Request) {
	if h.alertEngine == nil {
		http.Error(w, `{"error":"alert engine not available"}`, http.StatusServiceUnavailable)
		return
	}
	if err := h.alertEngine.SendTestTelegram(); err != nil {
		h.writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, map[string]bool{"ok": true})
}

func (h *Handler) testWebhook(w http.ResponseWriter, r *http.Request) {
	if h.alertEngine == nil {
		http.Error(w, `{"error":"alert engine not available"}`, http.StatusServiceUnavailable)
		return
	}
	if err := h.alertEngine.SendTestWebhook(); err != nil {
		h.writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	h.writeJSON(w, map[string]bool{"ok": true})
}
