package ami

// Event represents a parsed AMI event as key-value pairs
type Event map[string]string

// Get returns a value by key, or empty string
func (e Event) Get(key string) string {
	return e[key]
}

// EventType returns the Event header value
func (e Event) EventType() string {
	return e["Event"]
}

// ActionID returns the ActionID header value
func (e Event) ActionID() string {
	return e["ActionID"]
}

// Known event types
const (
	EventNewchannel    = "Newchannel"
	EventHangup        = "Hangup"
	EventDialBegin     = "DialBegin"
	EventDialEnd       = "DialEnd"
	EventBridgeEnter   = "BridgeEnter"
	EventBridgeLeave   = "BridgeLeave"
	EventRTCPSent      = "RTCPSent"
	EventRTCPReceived  = "RTCPReceived"
	EventContactStatus = "ContactStatus"
	EventPeerStatus    = "PeerStatus"
	EventFullyBooted   = "FullyBooted"
	EventEndpointList  = "EndpointList"
	EventContactList   = "ContactList"

	EventEndpointListComplete = "EndpointListComplete"
	EventContactListComplete  = "ContactListComplete"

	// Security events
	EventInvalidAccountID        = "InvalidAccountID"
	EventChallengeResponseFailed = "ChallengeResponseFailed"
	EventInvalidPassword         = "InvalidPassword"
	EventFailedACL               = "FailedACL"
	EventUnexpectedAddress       = "UnexpectedAddress"
	EventRequestBadFormat        = "RequestBadFormat"
)
