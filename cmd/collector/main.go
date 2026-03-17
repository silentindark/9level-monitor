package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/cors"
	"github.com/9level/9level-monitor/internal/alerts"
	"github.com/9level/9level-monitor/internal/ami"
	"github.com/9level/9level-monitor/internal/api"
	"github.com/9level/9level-monitor/internal/ari"
	"github.com/9level/9level-monitor/internal/collector"
	"github.com/9level/9level-monitor/internal/config"
	"github.com/9level/9level-monitor/internal/db"
	"github.com/9level/9level-monitor/internal/store"
)

func main() {
	cfg := config.Load()

	// Initialize SQLite database
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// Initialize components
	st := store.New()
	amiClient := ami.NewClient(cfg.AMIHost, cfg.AMIPort, cfg.AMIUser, cfg.AMISecret)
	ariClient := ari.NewClient(cfg.ARIBaseURL, cfg.ARIUser, cfg.ARIPass)
	broker := api.NewBroker()

	// Initialize alert engine
	alertEngine := alerts.New(database)

	// API
	handler := api.NewHandler(st, broker, database, amiClient.Connected, ariClient.Healthy, alertEngine)
	mux := http.NewServeMux()
	handler.Register(mux)

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "PUT", "POST"},
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: c.Handler(mux),
	}

	stopCh := make(chan struct{})

	// Start AMI client (reconnects automatically)
	go amiClient.Run(stopCh)

	// Start collector (processes AMI events + ARI polling)
	coll := collector.New(st, amiClient, ariClient, broker,
		cfg.RTPPollInterval, cfg.EndpointRefreshInterval, cfg.SecurityWhitelistIPs, database, alertEngine)

	go func() {
		// Wait for AMI connection before bootstrap
		for i := 0; i < 50; i++ {
			if amiClient.Connected() {
				break
			}
			select {
			case <-stopCh:
				return
			case <-time.After(100 * time.Millisecond):
			}
		}

		if amiClient.Connected() {
			collector.Bootstrap(st, ariClient, amiClient)
		} else {
			log.Println("main: AMI not connected after timeout, bootstrap skipped")
		}

		coll.Run(stopCh)
	}()

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		close(stopCh)
		srv.Close()
	}()

	log.Printf("9level-collector listening on :%s (AMI: %s:%s, ARI: %s, DB: %s)",
		cfg.Port, cfg.AMIHost, cfg.AMIPort, cfg.ARIBaseURL, cfg.DBPath)

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
