package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"pbs-exporter/internal/metrics"
	"pbs-exporter/internal/pbs"
	"pbs-exporter/internal/server"
)

func main() {
	listen := flag.String("listen", "0.0.0.0:8888", "Address to serve metrics on")
	interval := flag.Duration("interval", 60*time.Second, "How often to query PBS")
	commandTimeout := flag.Duration("command-timeout", 45*time.Second, "Time limit for each PBS command")
	flag.Parse()

	// Initialize metrics registry
	registry := metrics.NewRegistry()

	// Initialize PBS client
	pbsClient := pbs.NewClient()
	pbsClient.Timeout = *commandTimeout

	// Create and configure server
	srv := server.New(registry, pbsClient)

	// Start metrics collection in background
	go func() {
		// Update immediately on start
		srv.UpdateMetrics()

		ticker := time.NewTicker(*interval)
		defer ticker.Stop()

		for {
			<-ticker.C
			srv.UpdateMetrics()
		}
	}()

	// Start HTTP server
	log.Printf("PBS cluster monitoring server starting on %s", *listen)
	log.Printf("Metrics available at http://%s/metrics", *listen)
	log.Fatal(http.ListenAndServe(*listen, promhttp.HandlerFor(registry.GetRegistry(), promhttp.HandlerOpts{})))
}
