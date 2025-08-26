package main

import (
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"pbs-exporter/internal/metrics"
	"pbs-exporter/internal/pbs"
	"pbs-exporter/internal/server"
)

func main() {
	// Initialize metrics registry
	registry := metrics.NewRegistry()

	// Initialize PBS client
	pbsClient := pbs.NewClient()

	// Create and configure server
	srv := server.New(registry, pbsClient)

	// Start metrics collection in background
	go func() {
		// Update immediately on start
		srv.UpdateMetrics()

		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			<-ticker.C
			srv.UpdateMetrics()
		}
	}()

	// Start HTTP server
	log.Println("PBS cluster monitoring server starting on 0.0.0.0:8888")
	log.Println("Metrics available at http://0.0.0.0:8888/metrics")
	log.Fatal(http.ListenAndServe("0.0.0.0:8888", promhttp.HandlerFor(registry.GetRegistry(), promhttp.HandlerOpts{})))
}