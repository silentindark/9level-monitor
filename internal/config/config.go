package config

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	// AMI
	AMIHost   string
	AMIPort   string
	AMIUser   string
	AMISecret string

	// ARI
	ARIBaseURL string
	ARIUser    string
	ARIPass    string

	// Server
	Port string

	// Database
	DBPath string

	// Polling
	RTPPollInterval         time.Duration
	EndpointRefreshInterval time.Duration

	// Security
	SecurityWhitelistIPs []string // IPs to ignore in security events
}

func Load() *Config {
	c := &Config{
		AMIHost:   getEnv("AMI_HOST", "127.0.0.1"),
		AMIPort:   getEnv("AMI_PORT", "5038"),
		AMIUser:   getEnv("AMI_USER", "9level"),
		AMISecret: getEnv("AMI_SECRET", ""),

		ARIBaseURL: getEnv("ARI_BASE_URL", "http://127.0.0.1:8088/ari"),
		ARIUser:    getEnv("ARI_USER", "9level"),
		ARIPass:    getEnv("ARI_PASS", ""),

		Port:   getEnv("PORT", "3001"),
		DBPath: getEnv("DB_PATH", "/data/9level.db"),
	}

	if d, err := time.ParseDuration(getEnv("RTP_POLL_INTERVAL", "30s")); err == nil {
		c.RTPPollInterval = d
	} else {
		c.RTPPollInterval = 30 * time.Second
	}

	if d, err := time.ParseDuration(getEnv("ENDPOINT_REFRESH_INTERVAL", "5m")); err == nil {
		c.EndpointRefreshInterval = d
	} else {
		c.EndpointRefreshInterval = 5 * time.Minute
	}

	if wl := getEnv("SECURITY_WHITELIST_IPS", ""); wl != "" {
		for _, ip := range strings.Split(wl, ",") {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				c.SecurityWhitelistIPs = append(c.SecurityWhitelistIPs, ip)
			}
		}
	}

	return c
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
