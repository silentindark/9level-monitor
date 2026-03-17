package alerts

import (
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/9level/9level-monitor/internal/db"
)

// Engine evaluates alert rules and dispatches notifications
type Engine struct {
	mu sync.RWMutex
	db *db.DB

	// global
	enabled bool

	// MOS alerts
	mosThreshold float64
	mosCooldown  time.Duration

	// endpoint alerts
	endpointEnabled  bool
	endpointCooldown time.Duration

	// security alerts
	securityEnabled   bool
	securityThreshold int
	securityCooldown  time.Duration

	// channels
	telegramEnabled bool
	telegramToken   string
	telegramChatID  string
	webhookEnabled  bool
	webhookURL      string

	// cooldown trackers
	lastMOSAlert      time.Time
	lastEndpointAlert map[string]time.Time
	lastSecurityAlert map[string]time.Time
}

// New creates an Engine and loads settings from the database
func New(database *db.DB) *Engine {
	e := &Engine{
		db:                database,
		lastEndpointAlert: make(map[string]time.Time),
		lastSecurityAlert: make(map[string]time.Time),
	}
	e.Reload()
	return e
}

// Reload re-reads all alert settings from the database
func (e *Engine) Reload() {
	e.mu.Lock()
	defer e.mu.Unlock()

	settings, err := e.db.GetAllSettings()
	if err != nil {
		log.Printf("alerts: failed to load settings: %v", err)
		return
	}

	e.enabled = parseBool(settings["alerts.enabled"])
	e.mosThreshold = parseFloat(settings["alerts.mos_threshold"], 3.0)
	e.mosCooldown = parseDuration(settings["alerts.mos_cooldown"], 5*time.Minute)
	e.endpointEnabled = parseBool(settings["alerts.endpoint_enabled"])
	e.endpointCooldown = parseDuration(settings["alerts.endpoint_cooldown"], 5*time.Minute)
	e.securityEnabled = parseBool(settings["alerts.security_enabled"])
	e.securityThreshold = parseInt(settings["alerts.security_threshold"], 10)
	e.securityCooldown = parseDuration(settings["alerts.security_cooldown"], 5*time.Minute)
	e.telegramEnabled = parseBool(settings["telegram.enabled"])
	e.telegramToken = settings["telegram.bot_token"]
	e.telegramChatID = settings["telegram.chat_id"]
	e.webhookEnabled = parseBool(settings["webhook.enabled"])
	e.webhookURL = settings["webhook.url"]

	log.Printf("alerts: reloaded settings (enabled=%v)", e.enabled)
}

// CheckMOS fires an alert if either rxMOS or txMOS falls below the threshold
func (e *Engine) CheckMOS(channel string, rxMOS, txMOS float64) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.enabled {
		return
	}

	avg := (rxMOS + txMOS) / 2.0
	if avg >= e.mosThreshold {
		return
	}

	if time.Since(e.lastMOSAlert) < e.mosCooldown {
		return
	}

	subject := "Low MOS Alert"
	msg := fmt.Sprintf("*Low MOS detected*\nChannel: `%s`\nRx MOS: %.2f | Tx MOS: %.2f\nThreshold: %.1f",
		channel, rxMOS, txMOS, e.mosThreshold)

	e.dispatch(subject, msg)

	// upgrade to write lock for cooldown update
	e.mu.RUnlock()
	e.mu.Lock()
	e.lastMOSAlert = time.Now()
	e.mu.Unlock()
	e.mu.RLock()
}

// CheckEndpointDown fires an alert when an endpoint transitions to an unavailable state
func (e *Engine) CheckEndpointDown(endpoint, oldState, newState string) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.enabled || !e.endpointEnabled {
		return
	}

	if newState != "Unavailable" && newState != "Not in use" {
		return
	}

	if last, ok := e.lastEndpointAlert[endpoint]; ok && time.Since(last) < e.endpointCooldown {
		return
	}

	subject := "Endpoint Down"
	msg := fmt.Sprintf("*Endpoint state change*\nEndpoint: `%s`\n%s -> %s",
		endpoint, oldState, newState)

	e.dispatch(subject, msg)

	e.mu.RUnlock()
	e.mu.Lock()
	e.lastEndpointAlert[endpoint] = time.Now()
	e.mu.Unlock()
	e.mu.RLock()
}

// CheckSecurityFlood fires an alert when failed auth attempts from an IP exceed the threshold
func (e *Engine) CheckSecurityFlood(ip string, count int) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.enabled || !e.securityEnabled {
		return
	}

	if count < e.securityThreshold {
		return
	}

	if last, ok := e.lastSecurityAlert[ip]; ok && time.Since(last) < e.securityCooldown {
		return
	}

	subject := "Security Flood Alert"
	msg := fmt.Sprintf("*Security flood detected*\nIP: `%s`\nFailed attempts: %d (threshold: %d)",
		ip, count, e.securityThreshold)

	e.dispatch(subject, msg)

	e.mu.RUnlock()
	e.mu.Lock()
	e.lastSecurityAlert[ip] = time.Now()
	e.mu.Unlock()
	e.mu.RLock()
}

// SendTestTelegram sends a test message via Telegram
func (e *Engine) SendTestTelegram() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.telegramToken == "" || e.telegramChatID == "" {
		return fmt.Errorf("telegram bot_token or chat_id not configured")
	}
	return sendTelegram(e.telegramToken, e.telegramChatID, "*9Level Test*\nAlert engine is working.")
}

// SendTestWebhook sends a test message via webhook
func (e *Engine) SendTestWebhook() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.webhookURL == "" {
		return fmt.Errorf("webhook URL not configured")
	}
	return sendWebhook(e.webhookURL, "9Level Test", "Alert engine is working.")
}

// dispatch sends a message to all enabled channels
func (e *Engine) dispatch(subject, message string) {
	if e.telegramEnabled && e.telegramToken != "" && e.telegramChatID != "" {
		if err := sendTelegram(e.telegramToken, e.telegramChatID, message); err != nil {
			log.Printf("alerts: telegram error: %v", err)
		}
	}
	if e.webhookEnabled && e.webhookURL != "" {
		if err := sendWebhook(e.webhookURL, subject, message); err != nil {
			log.Printf("alerts: webhook error: %v", err)
		}
	}
}

// --- helpers ---

func parseBool(s string) bool {
	return s == "true" || s == "1"
}

func parseFloat(s string, def float64) float64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func parseDuration(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return v
}
