package collector

import (
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/9level/9level-monitor/internal/ami"
	"github.com/9level/9level-monitor/internal/ari"
	"github.com/9level/9level-monitor/internal/store"
)

// Bootstrap loads initial state from ARI and AMI
func Bootstrap(st *store.Store, ariClient *ari.Client, amiClient *ami.Client) {
	bootstrapChannels(st, ariClient)
	bootstrapEndpoints(st, amiClient)
}

func bootstrapChannels(st *store.Store, ariClient *ari.Client) {
	channels, err := ariClient.GetChannels()
	if err != nil {
		log.Printf("bootstrap: failed to get channels from ARI: %v", err)
		return
	}

	for _, ch := range channels {
		created, _ := time.Parse(time.RFC3339, ch.CreationTime)
		if created.IsZero() {
			created = time.Now()
		}

		cs := &store.ChannelState{
			Name:         ch.Name,
			UniqueID:     ch.ID,
			CallerNum:    ch.Caller.Number,
			ConnectedNum: ch.Connected.Number,
			CreationTime: created,
			State:        ch.State,
		}

		// Try to get RTP stats
		stats, err := ariClient.GetChannelRTPStats(url.PathEscape(ch.ID))
		if err == nil && stats != nil {
			cs.Quality = &store.QualityState{
				RxMES:      mesToMOS(stats.RxMES),
				TxMES:      mesToMOS(stats.TxMES),
				RxJitter:   jitterARItoMS(stats.RxJitter),
				TxJitter:   jitterARItoMS(stats.TxJitter),
				RxPLoss:    stats.RxPLoss,
				TxPLoss:    stats.TxPLoss,
				RTT:        rttToMS(stats.RTT),
				MaxRTT:     rttToMS(stats.MaxRTT),
				MinRTT:     rttToMS(stats.MinRTT),
				TxCount:    stats.TxCount,
				RxCount:    stats.RxCount,
				LocalSSRC:  stats.LocalSSRC,
				RemoteSSRC: stats.RemoteSSRC,
				UpdatedAt:  time.Now(),
			}
		}

		st.SetChannel(ch.Name, cs)
	}

	log.Printf("bootstrap: loaded %d channels from ARI", len(channels))
}

func bootstrapEndpoints(st *store.Store, amiClient *ami.Client) {
	if !amiClient.Connected() {
		log.Println("bootstrap: AMI not connected, skipping endpoint bootstrap")
		return
	}

	// Request PJSIPShowEndpoints via AMI
	err := amiClient.SendAction("PJSIPShowEndpoints", nil)
	if err != nil {
		log.Printf("bootstrap: failed to send PJSIPShowEndpoints: %v", err)
		return
	}

	// Endpoints will arrive as events and be processed by the collector event loop.
	// We also request contacts to populate URI/status/RTT.
	time.Sleep(500 * time.Millisecond) // give time for endpoint events to arrive

	err = amiClient.SendAction("PJSIPShowContacts", nil)
	if err != nil {
		log.Printf("bootstrap: failed to send PJSIPShowContacts: %v", err)
	}

	log.Println("bootstrap: requested PJSIPShowEndpoints + PJSIPShowContacts via AMI")
}

// ProcessEndpointListEvent handles EndpointList events from PJSIPShowEndpoints
func ProcessEndpointListEvent(st *store.Store, ev ami.Event) {
	name := ev.Get("ObjectName")
	if name == "" {
		return
	}

	ep := st.GetOrCreateEndpoint(name)
	deviceState := ev.Get("DeviceState")

	// "Not in use" or "In use" = registered, "Unavailable" = not registered
	if deviceState == "Unavailable" {
		ep.State = "OFFLINE"
	} else {
		ep.State = "ONLINE"
	}

	// Parse contacts if present in the event
	contacts := ev.Get("Contacts")
	if contacts != "" {
		contactStatus := "Available"
		if deviceState == "Unavailable" {
			contactStatus = "Unavailable"
		}
		parseContactsField(ep, contacts, contactStatus)
	}
}

// ProcessContactListEvent handles ContactList events from PJSIPShowContacts
func ProcessContactListEvent(st *store.Store, ev ami.Event) {
	uri := ev.Get("Uri")
	status := ev.Get("Status")

	// PJSIPShowContacts returns "Endpoint" field, not "Aor"
	epName := ev.Get("Endpoint")
	if epName == "" {
		epName = ev.Get("Aor")
	}

	if epName == "" || uri == "" {
		return
	}

	if idx := strings.Index(epName, "/"); idx >= 0 {
		epName = epName[:idx]
	}

	ep := st.GetOrCreateEndpoint(epName)
	if ep.Contacts == nil {
		ep.Contacts = make(map[string]*store.ContactState)
	}

	var rtt int64
	if v := ev.Get("RoundtripUsec"); v != "" {
		rtt, _ = strconv.ParseInt(v, 10, 64)
	}

	ep.Contacts[uri] = &store.ContactState{
		URI:       uri,
		Status:    normalizeContactStatus(status),
		RTT:       rtt,
		UpdatedAt: time.Now(),
	}
	ep.UpdateState()
}

func normalizeContactStatus(status string) string {
	switch strings.ToLower(status) {
	case "reachable", "available":
		return "Available"
	case "unreachable", "unavailable":
		return "Unavailable"
	default:
		return status
	}
}

// parseContactsField parses the Contacts field from EndpointList event
// Format: "aor/contact_uri,aor/contact_uri" or similar
func parseContactsField(ep *store.EndpointState, contacts string, status string) {
	if ep.Contacts == nil {
		ep.Contacts = make(map[string]*store.ContactState)
	}

	parts := strings.Split(contacts, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		ep.Contacts[part] = &store.ContactState{
			URI:       part,
			Status:    status,
			UpdatedAt: time.Now(),
		}
	}
}
