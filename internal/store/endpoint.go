package store

import "time"

// EndpointState holds the state of a PJSIP endpoint
type EndpointState struct {
	Name     string                   `json:"name"`
	State    string                   `json:"state"` // ONLINE or OFFLINE
	Contacts map[string]*ContactState `json:"contacts"`
}

// ContactState holds the state of an endpoint contact
type ContactState struct {
	URI       string    `json:"uri"`
	Status    string    `json:"status"` // Available, Unavailable, Unknown, etc.
	RTT       int64     `json:"rtt_us"` // microseconds
	UpdatedAt time.Time `json:"updated_at"`
}

// IsOnline returns true if at least one contact is Available
func (e *EndpointState) IsOnline() bool {
	for _, c := range e.Contacts {
		if c.Status == "Reachable" || c.Status == "Available" || c.Status == "Created" {
			return true
		}
	}
	return false
}

// UpdateState recomputes the ONLINE/OFFLINE state from contacts
func (e *EndpointState) UpdateState() {
	if e.IsOnline() {
		e.State = "ONLINE"
	} else {
		e.State = "OFFLINE"
	}
}
