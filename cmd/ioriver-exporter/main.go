package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	ioriver "github.com/ioriver/ioriver-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"ioriver_exporter/internal/cache"
	"ioriver_exporter/internal/collectors"
	"ioriver_exporter/internal/manager"
	exporter_settings "ioriver_exporter/internal/settings"
)

const (
	name    = "ioriver-exporter"
	version = "0.1.0"
)

func main() {

	// collect and validate application settings
	settings, err := exporter_settings.CollectSettings(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if settings.Version {
		fmt.Fprintf(os.Stdout, "%s v%s\n", name, version)
		os.Exit(0)
	}

	logger := log.NewLogfmtLogger(os.Stdout)
	logger = level.NewFilter(logger, getLogLevel(settings.Verbose))

	if !settings.Validate(logger) {
		os.Exit(1)
	}

	// create IORiver API client
	iorClient := ioriver.NewClient(settings.Token)

	// create the service cache and refresh it
	level.Info(logger).Log("main", "start polling", "interval_sec", settings.ServiceRefresh)
	serviceCache := cache.NewServiceCache(iorClient, logger)
	if err = serviceCache.Refresh(); err != nil {
		level.Warn(logger).Log("main", "failed to init the service cache")
	}

	// context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	r := prometheus.NewRegistry()
	collector := collectors.NewTrafficCollector(settings.TrafficTimestamp, logger)
	r.MustRegister(collector)

	http.Handle("/metrics", promhttp.HandlerFor(r, promhttp.HandlerOpts{Registry: r}))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(getLandingPage())
		if err != nil {
			level.Error(logger).Log("main", "the landing page failed")
		}
	})

	// The HTTP server that Prometheus will scrape.
	serverLogger := log.With(logger, "component", "server")
	server := http.Server{Addr: settings.Listen}

	ticker := time.NewTicker(settings.ServiceRefresh)
	manager := manager.NewSubscriptionManager(serviceCache, iorClient, collector, settings, logger)
	manager.Refresh()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		for {
			select {
			case <-ticker.C:
				err := serviceCache.Refresh()
				if err != nil {
					note := fmt.Sprintf("try again in %d sec", int(settings.ServiceRefresh.Seconds()))
					level.Warn(logger).Log("main", "failed to refresh the service cache", "note", note)
				}
				manager.Refresh()
			case <-ctx.Done():
				level.Info(logger).Log("main", "shutting down gracefully...")
				manager.StopAll()
				err = server.Shutdown(ctx)
				if err != nil {
					level.Info(logger).Log("main", "failed to shutdown the service gracefully")
				}
				wg.Done()
				return
			}
		}
	}()

	go func() {
		level.Info(serverLogger).Log("listen", settings.Listen)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			level.Error(serverLogger).Log("main", err)
		}
	}()

	wg.Wait()
}

func getLogLevel(debug bool) level.Option {
	if debug {
		return level.AllowDebug()
	}
	return level.AllowInfo()
}

func getLandingPage() []byte {
	return []byte(`<html>
		<head><title>ioriver-exporter</title></head>
		<body>
		<h1>ioriver-exporter</h1>
		<p><a href="/metrics">Metrics</a></p>
		</body>
		</html>`)
}
