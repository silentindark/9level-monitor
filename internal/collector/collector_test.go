package collector

import (
	"math"
	"testing"
)

func TestMesToMOS(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
		delta    float64
	}{
		{"zero gives 1.0", 0, 1.0, 0.01},
		{"negative gives 1.0", -10, 1.0, 0.01},
		{"mes 100 gives 4.5", 100, 4.5, 0.01},
		{"mes 72.6 gives ~3.72", 72.6, 3.72, 0.05},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mesToMOS(tt.input)
			if math.Abs(got-tt.expected) > tt.delta {
				t.Errorf("mesToMOS(%v) = %v, want %v (±%v)", tt.input, got, tt.expected, tt.delta)
			}
		})
	}
}

func TestJitterARItoMS(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{"32 units = 4ms", 32, 4.0},
		{"zero stays zero", 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jitterARItoMS(tt.input)
			if math.Abs(got-tt.expected) > 0.01 {
				t.Errorf("jitterARItoMS(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestJitterAMItoMS(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{"3ms stays 3ms", 3.0, 3.0},
		{"0.5ms stays 0.5ms", 0.5, 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jitterAMItoMS(tt.input)
			if math.Abs(got-tt.expected) > 0.01 {
				t.Errorf("jitterAMItoMS(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRttToMS(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{"0.0188s = 18.8ms", 0.0188, 18.8},
		{"zero stays zero", 0, 0.0},
		{"5.0 already in ms", 5.0, 5.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rttToMS(tt.input)
			if math.Abs(got-tt.expected) > 0.01 {
				t.Errorf("rttToMS(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseAsteriskAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid IPV4/UDP format", "IPV4/UDP/1.2.3.4/5060", "1.2.3.4:5060"},
		{"invalid falls through", "invalido", "invalido"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAsteriskAddress(tt.input)
			if got != tt.expected {
				t.Errorf("parseAsteriskAddress(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
