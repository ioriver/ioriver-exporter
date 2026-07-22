package collectors

import (
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
	metricsProvider  MetricsProvider
	trafficTimestamp bool
	logger           log.Logger
	mtx              sync.RWMutex
}

func NewTrafficCollector(trafficTimestamp bool, logger log.Logger) *TrafficCollector {
	return &TrafficCollector{
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
	provider := c.metricsProvider
	c.mtx.RUnlock()

	if provider == nil {
		return
	}

	metrics := provider.GetPrometheusMetrics()
	for _, m := range metrics {
		if c.trafficTimestamp {
			ch <- prometheus.NewMetricWithTimestamp(time.UnixMilli(m.Timestamp), *m.Metric)
		} else {
			ch <- *m.Metric
		}
	}
}

// Register the shared metrics provider.
func (c *TrafficCollector) RegisterMetricsProvider(provider MetricsProvider) {
	level.Debug(c.logger).Log("collector", "register metrics provider")

	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.metricsProvider = provider
}

// Unregister the shared metrics provider.
func (c *TrafficCollector) UnregisterMetricsProvider() {
	level.Debug(c.logger).Log("collector", "unregister metrics provider")

	c.mtx.Lock()
	defer c.mtx.Unlock()

	c.metricsProvider = nil
}
