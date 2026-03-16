package ari

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Channel represents an ARI channel
type Channel struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	State       string      `json:"state"`
	Caller      CallerID    `json:"caller"`
	Connected   CallerID    `json:"connected"`
	CreationTime string     `json:"creationtime"`
	Dialplan    Dialplan    `json:"dialplan"`
	Language    string      `json:"language"`
	ChannelVars map[string]string `json:"channelvars,omitempty"`
}

type CallerID struct {
	Name   string `json:"name"`
	Number string `json:"number"`
}

type Dialplan struct {
	Context  string `json:"context"`
	Exten    string `json:"exten"`
	Priority int    `json:"priority"`
	AppName  string `json:"app_name"`
	AppData  string `json:"app_data"`
}

// RTPStats represents ARI RTP statistics for a channel
type RTPStats struct {
	TxCount          int     `json:"txcount"`
	RxCount          int     `json:"rxcount"`
	TxJitter         float64 `json:"txjitter"`
	RxJitter         float64 `json:"rxjitter"`
	TxPLoss          int     `json:"txploss"`
	RxPLoss          int     `json:"rxploss"`
	RTT              float64 `json:"rtt"`
	MaxRTT           float64 `json:"maxrtt"`
	MinRTT           float64 `json:"minrtt"`
	NormDevRTT       float64 `json:"normdevrtt"`
	StDevRTT         float64 `json:"stdevrtt"`
	TxMES            float64 `json:"txmes"`
	RxMES            float64 `json:"rxmes"`
	RemoteMaxJitter  float64 `json:"remote_maxjitter"`
	RemoteMinJitter  float64 `json:"remote_minjitter"`
	LocalMaxJitter   float64 `json:"local_maxjitter"`
	LocalMinJitter   float64 `json:"local_minjitter"`
	RemoteMaxRxPLoss float64 `json:"remote_maxrxploss"`
	RemoteMinRxPLoss float64 `json:"remote_minrxploss"`
	LocalMaxRxPLoss  float64 `json:"local_maxrxploss"`
	LocalMinRxPLoss  float64 `json:"local_minrxploss"`
	LocalSSRC        int     `json:"local_ssrc"`
	RemoteSSRC       int     `json:"remote_ssrc"`
	TxOctetCount     int     `json:"txoctetcount"`
	RxOctetCount     int     `json:"rxoctetcount"`
	LocalMaxMES      float64 `json:"local_maxmes"`
	LocalMinMES      float64 `json:"local_minmes"`
	LocalNormDevMES  float64 `json:"local_normdevmes"`
	RemoteMaxMES     float64 `json:"remote_maxmes"`
	RemoteMinMES     float64 `json:"remote_minmes"`
	RemoteNormDevMES float64 `json:"remote_normdevmes"`
}

// Endpoint represents an ARI endpoint
type Endpoint struct {
	Technology string   `json:"technology"`
	Resource   string   `json:"resource"`
	State      string   `json:"state"`
	ChannelIDs []string `json:"channel_ids"`
}

// Client is an HTTP client for the Asterisk REST Interface
type Client struct {
	baseURL    string
	httpClient *http.Client
	user       string
	pass       string
}

func NewClient(baseURL, user, pass string) *Client {
	return &Client{
		baseURL: baseURL,
		user:    user,
		pass:    pass,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) get(path string, result any) error {
	url := c.baseURL + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.SetBasicAuth(c.user, c.pass)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d: %s", path, resp.StatusCode, string(body))
	}

	return json.Unmarshal(body, result)
}

// GetChannels returns all active channels
func (c *Client) GetChannels() ([]Channel, error) {
	var channels []Channel
	err := c.get("/channels", &channels)
	return channels, err
}

// GetChannelRTPStats returns RTP statistics for a specific channel
func (c *Client) GetChannelRTPStats(channelID string) (*RTPStats, error) {
	var stats RTPStats
	err := c.get("/channels/"+channelID+"/rtp_statistics", &stats)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetEndpoints returns all endpoints
func (c *Client) GetEndpoints() ([]Endpoint, error) {
	var endpoints []Endpoint
	err := c.get("/endpoints", &endpoints)
	return endpoints, err
}

// Healthy checks if ARI is reachable
func (c *Client) Healthy() bool {
	var info map[string]any
	return c.get("/asterisk/info", &info) == nil
}
