package subscriber

import (
	"context"
	"ioriver_exporter/api"
	"ioriver_exporter/internal/collectors"
	"ioriver_exporter/internal/metrics"
	"slices"
	"sync"
	"time"
)

// Subscriber polls IORiver traffic statistics endpoints for a given service
// and keep it as Prometheus metrics.
type Subscriber struct {
	iorTraffic *IORiverTraffic
	services   []api.ServiceInfo
	metrics    []metrics.Metrics
	interval   time.Duration

	mtx sync.RWMutex
}

func NewSubscriber(iorTraffic *IORiverTraffic, interval time.Duration) *Subscriber {
	return &Subscriber{
		iorTraffic: iorTraffic,
		services:   make([]api.ServiceInfo, 0),
		metrics:    make([]metrics.Metrics, 0),
		interval:   interval,
	}
}

// UpdateServices atomically replaces the current service list.
func (s *Subscriber) UpdateServices(services []api.ServiceInfo) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.services = slices.Clone(services)
}

// Subscribe starts getting IORiver traffic statistic and building Prometheus metrics.
func (s *Subscriber) Subscribe(ctx context.Context) error {
	s.updateMetrics()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-time.After(s.interval):
			s.updateMetrics()
		}
	}
}

func (s *Subscriber) updateMetrics() {
	s.mtx.RLock()
	services := slices.Clone(s.services)
	s.mtx.RUnlock()

	// If there are no services to scrape anymore, clear previously exported metrics.
	if len(services) == 0 {
		s.mtx.Lock()
		s.metrics = nil
		s.mtx.Unlock()
		return
	}

	allMetrics := s.iorTraffic.GetTrafficMetrics(services)
	if len(allMetrics) == 0 {
		return
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.metrics = allMetrics
}

// Get the recent Prometheus traffic metrics of the subscribed service.
func (s *Subscriber) GetPrometheusMetrics() []collectors.PrometheusMetric {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	metrics := make([]collectors.PrometheusMetric, 0, len(s.metrics))
	for _, m := range s.metrics {
		for _, pm := range m.ToPrometheusMetrics() {
			metrics = append(metrics, collectors.PrometheusMetric{Metric: pm, Timestamp: m.GetTimestamp()})
		}
	}
	return metrics
}
