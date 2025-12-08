package collectors

import (
	"fmt"
	"sync"
	"time"

	"ioriver_exporter/internal/metrics"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

type PrometheusMetric struct {
	Metric    *prometheus.Metric
	Timestamp int64
}

// MetricsProvider is a contract for a metrics provider.
type MetricsProvider interface {
	GetPrometheusMetrics() []PrometheusMetric
}

type TrafficCollector struct {
	metricsProviders map[string]MetricsProvider
	trafficTimestamp bool
	logger           log.Logger
	mtx              sync.RWMutex
}

func NewTrafficCollector(trafficTimestamp bool, logger log.Logger) *TrafficCollector {
	return &TrafficCollector{
		metricsProviders: map[string]MetricsProvider{},
		trafficTimestamp: trafficTimestamp,
		logger:           logger,
	}
}

func (c *TrafficCollector) Describe(ch chan<- *prometheus.Desc) {
	name := prometheus.BuildFQName(metrics.Namespace, metrics.Subsystem, "traffic-collector")
	ch <- prometheus.NewDesc(name, "IORiver service traffic", nil, nil)
}

// Collect collects Prometheus metrics from all registered metrics providers.
func (c *TrafficCollector) Collect(ch chan<- prometheus.Metric) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	for _, metricsProvider := range c.metricsProviders {
		metrics := metricsProvider.GetPrometheusMetrics()
		for _, m := range metrics {
			if c.trafficTimestamp {
				ch <- prometheus.NewMetricWithTimestamp(time.UnixMilli(m.Timestamp), *m.Metric)
			} else {
				ch <- *m.Metric
			}
		}
	}
}

// Register a metric provider.
func (c *TrafficCollector) RegisterMetricsProvider(serviceId string, provider MetricsProvider) {
	level.Debug(c.logger).Log("collector", fmt.Sprintf("register metrics provider for %s", serviceId))

	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.metricsProviders[serviceId] = provider
}

// Unregister the metric provider.
func (c *TrafficCollector) UnregisterMetricsProvider(serviceId string) {
	level.Debug(c.logger).Log("collector", fmt.Sprintf("unregister metrics provider for %s", serviceId))

	c.mtx.Lock()
	defer c.mtx.Unlock()

	delete(c.metricsProviders, serviceId)
}
