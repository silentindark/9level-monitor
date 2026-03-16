package store

import "time"

// ChannelState holds the state of an active channel
type ChannelState struct {
	Name         string       `json:"channel"`
	UniqueID     string       `json:"uniqueid"`
	CallerNum    string       `json:"caller"`
	ConnectedNum string       `json:"callee"`
	CreationTime time.Time    `json:"creation_time"`
	Duration     int          `json:"duration_seconds"`
	Codec        string       `json:"codec,omitempty"`
	BridgeID     string       `json:"bridge_id,omitempty"`
	LinkedChannel string      `json:"linked_channel,omitempty"`
	State        string       `json:"state"`
	Quality      *QualityState `json:"rtp"`
}

// QualityState holds RTP quality metrics for a channel
type QualityState struct {
	RxMES      float64   `json:"rxmes"`
	TxMES      float64   `json:"txmes"`
	RxJitter   float64   `json:"rxjitter"`
	TxJitter   float64   `json:"txjitter"`
	RxPLoss    int       `json:"rxploss"`
	TxPLoss    int       `json:"txploss"`
	RTT        float64   `json:"rtt"`
	MaxRTT     float64   `json:"maxrtt"`
	MinRTT     float64   `json:"minrtt"`
	TxCount    int       `json:"txcount"`
	RxCount    int       `json:"rxcount"`
	LocalSSRC  int       `json:"local_ssrc"`
	RemoteSSRC int       `json:"remote_ssrc"`
	UpdatedAt  time.Time `json:"updated_at"`
}
