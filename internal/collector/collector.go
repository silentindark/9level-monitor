package collector

import (
	"fmt"
	"log"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/9level/9level-monitor/internal/alerts"
	"github.com/9level/9level-monitor/internal/ami"
	"github.com/9level/9level-monitor/internal/api"
	"github.com/9level/9level-monitor/internal/ari"
	"github.com/9level/9level-monitor/internal/db"
	"github.com/9level/9level-monitor/internal/store"
)

// Collector processes AMI events and updates the store
type Collector struct {
	store       *store.Store
	amiClient   *ami.Client
	ariClient   *ari.Client
	broker      *api.Broker
	db          *db.DB
	alertEngine *alerts.Engine

	rtpPollInterval         time.Duration
	endpointRefreshInterval time.Duration
	securityWhitelistIPs    map[string]bool
}

func New(st *store.Store, amiC *ami.Client, ariC *ari.Client, broker *api.Broker,
	rtpInterval, epRefresh time.Duration, whitelistIPs []string, database *db.DB, alertEng *alerts.Engine) *Collector {
	wl := make(map[string]bool)
	for _, ip := range whitelistIPs {
		wl[ip] = true
	}
	return &Collector{
		store:                   st,
		amiClient:               amiC,
		ariClient:               ariC,
		broker:                  broker,
		db:                      database,
		alertEngine:             alertEng,
		rtpPollInterval:         rtpInterval,
		endpointRefreshInterval: epRefresh,
		securityWhitelistIPs:    wl,
	}
}

// Run starts the collector event loop. Blocks until stopCh is closed.
func (c *Collector) Run(stopCh <-chan struct{}) {
	events := c.amiClient.Events()
	rtpTicker := time.NewTicker(c.rtpPollInterval)
	epTicker := time.NewTicker(c.endpointRefreshInterval)
	summaryTicker := time.NewTicker(1 * time.Second)
	purgeTicker := time.NewTicker(1 * time.Hour) // check purge every hour
	dbPurgeTicker := time.NewTicker(24 * time.Hour)
	defer rtpTicker.Stop()
	defer epTicker.Stop()
	defer summaryTicker.Stop()
	defer purgeTicker.Stop()
	defer dbPurgeTicker.Stop()

	for {
		select {
		case <-stopCh:
			return

		case ev := <-events:
			start := time.Now()
			c.handleEvent(ev)
			amiMS := float64(time.Since(start).Microseconds()) / 1000.0
			c.store.UpdateLatency(amiMS, -1, -1, len(events))

		case <-rtpTicker.C:
			c.pollRTPStats()

		case <-epTicker.C:
			c.refreshEndpoints()

		case <-summaryTicker.C:
			c.broker.Send("summary:update", c.store.GetSummary())
			healthData := map[string]any{
				"ami_ms":         c.store.GetLatency().AMIMS,
				"ari_ms":         c.store.GetLatency().ARIMS,
				"rtp_poll_ms":    c.store.GetLatency().RTPPollMS,
				"events_per_sec": c.store.GetLatency().EventsPerSec,
				"ami_queue_len":  c.store.GetLatency().AMIQueueLen,
				"sse":            c.broker.ClientCount(),
			}
			if c.db != nil {
				if size, err := c.db.Size(); err == nil {
					healthData["db_size"] = humanSize(size)
				}
			}
			c.broker.Send("health:update", healthData)
			if len(c.store.SecurityEvents) > 0 {
				c.broker.Send("security:summary", c.store.GetSecuritySummary())
			}

		case <-purgeTicker.C:
			if n := c.store.PurgeSecurityEvents(12 * time.Hour); n > 0 {
				log.Printf("collector: purged %d security events older than 12h", n)
			}

		case <-dbPurgeTicker.C:
			if c.db != nil {
				if n, err := c.db.Purge(30); err != nil {
					log.Printf("collector: db purge error: %v", err)
				} else if n > 0 {
					log.Printf("collector: db purged %d records older than 30 days", n)
				}
			}
		}
	}
}

func (c *Collector) handleEvent(ev ami.Event) {
	switch ev.EventType() {
	case ami.EventNewchannel:
		c.onNewchannel(ev)
	case ami.EventHangup:
		c.onHangup(ev)
	case ami.EventDialBegin:
		c.onDialBegin(ev)
	case ami.EventBridgeEnter:
		c.onBridgeEnter(ev)
	case ami.EventBridgeLeave:
		c.onBridgeLeave(ev)
	case ami.EventRTCPSent, ami.EventRTCPReceived:
		c.onRTCP(ev)
	case ami.EventContactStatus:
		c.onContactStatus(ev)
	case ami.EventPeerStatus:
		c.onPeerStatus(ev)
	case ami.EventEndpointList:
		ProcessEndpointListEvent(c.store, ev)
	case ami.EventContactList:
		ProcessContactListEvent(c.store, ev)
	case ami.EventInvalidAccountID,
		ami.EventChallengeResponseFailed,
		ami.EventInvalidPassword,
		ami.EventFailedACL,
		ami.EventUnexpectedAddress,
		ami.EventRequestBadFormat:
		c.onSecurityEvent(ev)
	}
}

func (c *Collector) onNewchannel(ev ami.Event) {
	name := ev.Get("Channel")
	if name == "" {
		return
	}

	ch := &store.ChannelState{
		Name:         name,
		UniqueID:     ev.Get("Uniqueid"),
		CallerNum:    ev.Get("CallerIDNum"),
		ConnectedNum: ev.Get("ConnectedLineNum"),
		CreationTime: time.Now(),
		State:        ev.Get("ChannelState"),
	}

	c.store.SetChannel(name, ch)
	c.broker.Send("call:new", ch)
	log.Printf("collector: new channel %s", name)
}

func (c *Collector) onHangup(ev ami.Event) {
	name := ev.Get("Channel")
	if name == "" {
		return
	}

	ch := c.store.RemoveChannel(name)
	if ch != nil {
		ch.Duration = int(time.Since(ch.CreationTime).Seconds())
		c.broker.Send("call:end", ch)

		// Persist call quality to SQLite (skip short calls < 7s, no real conversation)
		if c.db != nil && ch.Duration >= 7 {
			q := ch.Quality
			if q == nil {
				q = &store.QualityState{}
			}
			if err := c.db.InsertCallQuality(
				ch.Name, ch.UniqueID, ch.CallerNum, ch.ConnectedNum, ch.LinkedChannel, ch.Codec,
				ch.Duration, q.RxMES, q.TxMES, q.RxJitter, q.TxJitter, q.RxPLoss, q.TxPLoss,
				q.RTT, q.MaxRTT, q.MinRTT, q.TxCount, q.RxCount,
				ch.CreationTime, time.Now(),
			); err != nil {
				log.Printf("collector: db insert call_quality error: %v", err)
			} else {
				log.Printf("collector: db saved call %s duration=%ds mos=%.2f/%.2f", ch.Name, ch.Duration, q.RxMES, q.TxMES)
			}
		}

		if c.alertEngine != nil && ch.Quality != nil {
			c.alertEngine.CheckMOS(ch.Name, ch.Quality.RxMES, ch.Quality.TxMES)
		}
	}
	log.Printf("collector: hangup %s", name)
}

func (c *Collector) onDialBegin(ev ami.Event) {
	caller := ev.Get("Channel")
	callee := ev.Get("DestChannel")

	if ch := c.store.GetChannel(caller); ch != nil {
		ch.LinkedChannel = callee
		ch.ConnectedNum = ev.Get("DestCallerIDNum")
	}
	if ch := c.store.GetChannel(callee); ch != nil {
		ch.LinkedChannel = caller
		ch.CallerNum = ev.Get("CallerIDNum")
	}
}

func (c *Collector) onBridgeEnter(ev ami.Event) {
	bridgeID := ev.Get("BridgeUniqueid")
	chanName := ev.Get("Channel")
	if bridgeID == "" || chanName == "" {
		return
	}

	c.store.AddToBridge(bridgeID, chanName)
	if ch := c.store.GetChannel(chanName); ch != nil {
		ch.BridgeID = bridgeID
	}
}

func (c *Collector) onBridgeLeave(ev ami.Event) {
	bridgeID := ev.Get("BridgeUniqueid")
	chanName := ev.Get("Channel")
	if bridgeID == "" || chanName == "" {
		return
	}
	c.store.RemoveFromBridge(bridgeID, chanName)
}

func (c *Collector) onRTCP(ev ami.Event) {
	chanName := ev.Get("Channel")
	if chanName == "" {
		return
	}

	ch := c.store.GetChannel(chanName)
	if ch == nil {
		return
	}

	if ch.Quality == nil {
		ch.Quality = &store.QualityState{}
	}

	// Parse RTCP fields — MES comes as 0-100 from Asterisk, convert to MOS 1-4.5
	if v := ev.Get("MES"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			mos := mesToMOS(f)
			if ev.EventType() == ami.EventRTCPSent {
				ch.Quality.TxMES = mos
			} else {
				ch.Quality.RxMES = mos
			}
		}
	}

	if v := ev.Get("RTT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			ch.Quality.RTT = rttToMS(f)
		}
	}

	// Parse report block fields (Report0*)
	// AMI Report0IAJitter is in seconds (e.g. 0.004000 = 4ms)
	if v := ev.Get("Report0IAJitter"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			ms := jitterAMItoMS(f)
			if ev.EventType() == ami.EventRTCPSent {
				ch.Quality.TxJitter = ms
			} else {
				ch.Quality.RxJitter = ms
			}
		}
	}

	if v := ev.Get("Report0CumulativeLost"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if ev.EventType() == ami.EventRTCPSent {
				ch.Quality.TxPLoss = n
			} else {
				ch.Quality.RxPLoss = n
			}
		}
	}

	if v := ev.Get("SentPackets"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			ch.Quality.TxCount = n
		}
	}

	if v := ev.Get("SSRC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			ch.Quality.LocalSSRC = n
		}
	}

	ch.Quality.UpdatedAt = time.Now()
	c.broker.Send("call:update", ch)
}

func (c *Collector) onContactStatus(ev ami.Event) {
	uri := ev.Get("URI")
	contactStatus := ev.Get("ContactStatus")
	aor := ev.Get("AOR")
	rttStr := ev.Get("RoundtripUsec")

	if aor == "" {
		return
	}

	// Extract endpoint name from AOR
	epName := aor
	if idx := strings.Index(aor, "/"); idx >= 0 {
		epName = aor[:idx]
	}

	ep := c.store.GetOrCreateEndpoint(epName)
	if ep.Contacts == nil {
		ep.Contacts = make(map[string]*store.ContactState)
	}

	var rtt int64
	if rttStr != "" {
		rtt, _ = strconv.ParseInt(rttStr, 10, 64)
	}

	status := normalizeContactStatus(contactStatus)

	// Capture old state before update for change detection
	oldState := ep.State

	if status == "Removed" {
		delete(ep.Contacts, uri)
	} else if uri != "" {
		ep.Contacts[uri] = &store.ContactState{
			URI:       uri,
			Status:    status,
			RTT:       rtt,
			UpdatedAt: time.Now(),
		}
	}

	ep.UpdateState()

	// Detect state change and persist
	if oldState != ep.State {
		c.persistEndpointChange(epName, oldState, ep.State)
	}

	c.broker.Send("endpoint:update", ep)
}

func (c *Collector) onPeerStatus(ev ami.Event) {
	peer := ev.Get("Peer")
	status := ev.Get("PeerStatus")

	if peer == "" {
		return
	}

	// Peer format: "PJSIP/1001"
	epName := peer
	if idx := strings.Index(peer, "/"); idx >= 0 {
		epName = peer[idx+1:]
	}

	ep := c.store.GetOrCreateEndpoint(epName)

	// Capture old state before update
	oldState := ep.State

	switch status {
	case "Registered", "Reachable":
		ep.State = "ONLINE"
	case "Unregistered", "Unreachable":
		ep.State = "OFFLINE"
	}

	// Detect state change and persist
	if oldState != ep.State {
		c.persistEndpointChange(epName, oldState, ep.State)
	}

	c.broker.Send("endpoint:update", ep)
}

func (c *Collector) persistEndpointChange(endpoint, oldState, newState string) {
	now := time.Now()

	// Broadcast SSE event
	c.broker.Send("endpoint:state_change", map[string]string{
		"endpoint":  endpoint,
		"old_state": oldState,
		"new_state": newState,
		"timestamp": now.Format(time.RFC3339),
	})

	log.Printf("collector: endpoint %s state change %s → %s", endpoint, oldState, newState)

	// Persist to DB
	if c.db != nil {
		if err := c.db.InsertEndpointChange(endpoint, oldState, newState, now); err != nil {
			log.Printf("collector: db insert endpoint_change error: %v", err)
		}
	}

	if c.alertEngine != nil {
		c.alertEngine.CheckEndpointDown(endpoint, oldState, newState)
	}
}

func (c *Collector) onSecurityEvent(ev ami.Event) {
	addr := parseAsteriskAddress(ev.Get("RemoteAddress"))

	// Check whitelist — extract IP (without port)
	ip := addr
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		ip = addr[:idx]
	}
	if c.securityWhitelistIPs[ip] {
		return
	}

	secEv := &store.SecurityEvent{
		EventType:     ev.EventType(),
		AccountID:     ev.Get("AccountID"),
		RemoteAddress: addr,
		Service:       ev.Get("Service"),
		Timestamp:     time.Now(),
	}

	c.store.AddSecurityEvent(secEv)
	c.broker.Send("security:event", secEv)

	// Persist to DB
	if c.db != nil {
		if err := c.db.InsertSecurityEvent(secEv.EventType, secEv.AccountID, secEv.RemoteAddress, secEv.Service, secEv.Timestamp); err != nil {
			log.Printf("collector: db insert security_event error: %v", err)
		}
	}

	if c.alertEngine != nil {
		c.alertEngine.CheckSecurityFlood(secEv.RemoteAddress, c.store.GetSecurityCountByIP(secEv.RemoteAddress))
	}

	log.Printf("collector: SECURITY %s from %s (account: %s)",
		secEv.EventType, secEv.RemoteAddress, secEv.AccountID)
}

// parseAsteriskAddress extracts IP:port from Asterisk format "IPV4/UDP/1.2.3.4/5060"
func parseAsteriskAddress(raw string) string {
	parts := strings.Split(raw, "/")
	if len(parts) >= 4 {
		return parts[2] + ":" + parts[3]
	}
	return raw
}

// pollRTPStats fetches deep RTP stats from ARI for all active channels
func (c *Collector) pollRTPStats() {
	names := c.store.ChannelNames()
	if len(names) == 0 {
		return
	}

	pollStart := time.Now()
	var lastARIMS float64

	for _, name := range names {
		ch := c.store.GetChannel(name)
		if ch == nil {
			continue
		}

		reqStart := time.Now()
		stats, err := c.ariClient.GetChannelRTPStats(url.PathEscape(ch.UniqueID))
		lastARIMS = float64(time.Since(reqStart).Microseconds()) / 1000.0
		if err != nil {
			// Channel may not have RTP (Local channels, etc.)
			continue
		}

		if ch.Quality == nil {
			ch.Quality = &store.QualityState{}
		}

		ch.Quality.RxMES = mesToMOS(stats.RxMES)
		ch.Quality.TxMES = mesToMOS(stats.TxMES)
		ch.Quality.RxJitter = jitterARItoMS(stats.RxJitter)
		ch.Quality.TxJitter = jitterARItoMS(stats.TxJitter)
		ch.Quality.RxPLoss = stats.RxPLoss
		ch.Quality.TxPLoss = stats.TxPLoss
		ch.Quality.RTT = rttToMS(stats.RTT)
		ch.Quality.MaxRTT = rttToMS(stats.MaxRTT)
		ch.Quality.MinRTT = rttToMS(stats.MinRTT)
		ch.Quality.TxCount = stats.TxCount
		ch.Quality.RxCount = stats.RxCount
		ch.Quality.LocalSSRC = stats.LocalSSRC
		ch.Quality.RemoteSSRC = stats.RemoteSSRC
		ch.Quality.UpdatedAt = time.Now()
	}

	totalMS := float64(time.Since(pollStart).Microseconds()) / 1000.0
	c.store.UpdateLatency(-1, lastARIMS, totalMS, -1)
}

// jitterARItoMS converts jitter from ARI (RTP timestamp units at 8kHz) to ms.
func jitterARItoMS(jitter float64) float64 {
	// ARI returns jitter in timestamp units for 8kHz clock = divide by 8
	return math.Round(jitter/8*100) / 100
}

// jitterAMItoMS normalizes jitter from AMI Report0IAJitter.
// AMI already sends jitter in milliseconds as a float (e.g., 3.000000 = 3ms).
func jitterAMItoMS(jitter float64) float64 {
	return math.Round(jitter*100) / 100
}

// rttToMS converts RTT to milliseconds.
// AMI/ARI return RTT in seconds as a float (e.g., 0.0188 = 18.8ms).
// If value is already > 1, assume it's already in ms.
func rttToMS(rtt float64) float64 {
	if rtt <= 0 {
		return 0
	}
	if rtt < 1 {
		// In seconds, convert to ms
		return math.Round(rtt*1000*100) / 100
	}
	// Already in ms
	return math.Round(rtt*100) / 100
}

// mesToMOS converts Asterisk MES (0-100 scale) to MOS-like (1.0-4.5 scale)
// using the ITU-T G.107 simplified formula: MOS = 1 + 0.035*R + R*(R-60)*(100-R)*7e-6
func mesToMOS(mes float64) float64 {
	if mes <= 0 {
		return 1.0
	}
	if mes > 100 {
		mes = 100
	}
	mos := 1.0 + 0.035*mes + mes*(mes-60)*(100-mes)*7e-6
	// Clamp to valid MOS range
	if mos < 1.0 {
		return 1.0
	}
	if mos > 4.5 {
		return 4.5
	}
	return math.Round(mos*100) / 100
}

func humanSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// refreshEndpoints re-bootstraps endpoint state via AMI
func (c *Collector) refreshEndpoints() {
	if !c.amiClient.Connected() {
		return
	}
	log.Println("collector: refreshing endpoints via AMI")
	c.amiClient.SendAction("PJSIPShowEndpoints", nil)
	time.Sleep(500 * time.Millisecond)
	c.amiClient.SendAction("PJSIPShowContacts", nil)
}
