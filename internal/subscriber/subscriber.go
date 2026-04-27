package subscriber

import (
	"context"
	"fmt"
	"ioriver_exporter/api"
	"ioriver_exporter/internal/collectors"
	"ioriver_exporter/internal/metrics"
	"slices"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/ioriver/ioriver-go"
)

const updateInterval = 30 * time.Second
const metricsGranularity = time.Duration(-2) * time.Minute

// Subscriber polls IORiver traffic statistics endpoints for a given service
// and keep it as Prometheus metrics.
type Subscriber struct {
	iorClient    api.IORiverClient
	serviceId    string
	metrics      []metrics.Metrics
	trafficDelay time.Duration
	logger       log.Logger

	mtx sync.RWMutex
}

func NewSubscriber(iorClient api.IORiverClient, serviceId string, trafficDelay time.Duration, logger log.Logger) *Subscriber {
	return &Subscriber{
		iorClient:    iorClient,
		serviceId:    serviceId,
		metrics:      make([]metrics.Metrics, 0),
		trafficDelay: trafficDelay,
		logger:       logger,
	}
}

// Subscribe starts getting IORiver traffic statistic and building Prometheus metrics.
func (s *Subscriber) Subscribe(ctx context.Context) error {
	s.updateMetrics()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-time.After(updateInterval):
			s.updateMetrics()
		}
	}
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

func (s *Subscriber) updateMetrics() {
	allMetrics := make([]metrics.Metrics, 0, 4)
	var metrics []metrics.Metrics

	metrics = s.getTrafficMetrics()
	allMetrics = append(allMetrics, metrics...)

	metrics = s.getAdvancedTrafficMetrics(ioriver.StatusCode)
	allMetrics = append(allMetrics, metrics...)

	metrics = s.getAdvancedTrafficMetrics(ioriver.HttpVersion)
	allMetrics = append(allMetrics, metrics...)

	metrics = s.getAdvancedTrafficMetrics(ioriver.HttpMethod)
	allMetrics = append(allMetrics, metrics...)

	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.metrics = allMetrics
}

func (s *Subscriber) getTrafficMetrics() []metrics.Metrics {
	from, to := s.getTimeRange()
	traffic, err := s.iorClient.GetTraffic(s.serviceId, from.UnixMilli(), to.UnixMilli(), ioriver.Minute)
	if err != nil {
		level.Warn(s.logger).Log("subscriber", fmt.Sprintf("failed to get traffic for service %s: %s", s.serviceId, err))
		return []metrics.Metrics{}
	}

	maxTimestamp := s.getMaxTimestamp(s.serviceId, traffic.ServiceStats)
	if maxTimestamp == 0 {
		level.Debug(s.logger).Log("subscriber", fmt.Sprintf("no statistic points for service %s", s.serviceId))
		return []metrics.Metrics{}
	}

	// convert all stats metrics
	var metrics []metrics.Metrics
	for _, providerName := range getAllProviderNames(traffic.ServiceStats, s.serviceId) {
		providerMetrics := s.convertStatsToMetrics(traffic, providerName, maxTimestamp)
		metrics = append(metrics, providerMetrics...)
	}
	return metrics
}

func (s *Subscriber) getAdvancedTrafficMetrics(advancedMetric ioriver.AdvancedMetric) []metrics.Metrics {
	from, to := s.getTimeRange()
	traffic, err := s.iorClient.GetAdvancedTraffic(s.serviceId, from.UnixMilli(), to.UnixMilli(), ioriver.Minute, advancedMetric)
	if err != nil {
		level.Warn(s.logger).Log("subscriber", fmt.Sprintf("failed to get advanced traffic for service %s: %s", s.serviceId, err))
		return []metrics.Metrics{}
	}

	maxTimestamp := s.getMaxTimestamp(s.serviceId, traffic.ServiceStats)
	if maxTimestamp == 0 {
		level.Debug(s.logger).Log("subscriber", fmt.Sprintf("no statistic points for service %s", s.serviceId))
		return []metrics.Metrics{}
	}

	// convert all advanced stats metrics
	var metrics []metrics.Metrics
	for _, providerName := range getAllProviderNames(traffic.ServiceStats, s.serviceId) {
		providerMetrics := s.convertAdvancedStatsToMetrics(traffic, providerName, maxTimestamp, advancedMetric)
		metrics = append(metrics, providerMetrics...)
	}
	return metrics
}

func (s *Subscriber) convertStatsToMetrics(traffic *ioriver.Traffic, providerName string, timestamp int64) []metrics.Metrics {
	values := traffic.GetFilteredMetrics(s.serviceId, func(metric *ioriver.Metric, metricTimestamp int64) bool {
		return metric.ProviderName == providerName && metricTimestamp == timestamp
	})

	labels := map[string]string{"serviceID": s.serviceId, "providerName": abbreviationToProviderName(providerName)}
	level.Debug(s.logger).Log("service_id", s.serviceId, "provider", providerName, "time", timestamp, "subscriber", "update")
	providerMetrics := make([]metrics.Metrics, 0, len(values))

	for _, value := range values {
		stat := value.Metrics
		metrics := metrics.NewAllMetrics(labels, timestamp)
		metrics.Hits.Value = float64(stat.Hits)
		metrics.Bytes.Value = float64(stat.Bytes)
		metrics.CachedHitsPercentage.Value = stat.CachedHitsPercentage
		metrics.CachedBytesPercentage.Value = stat.CachedBytesPercentage
		metrics.ErrorsPercentage.Value = stat.ErrorsPercentage

		providerMetrics = append(providerMetrics, metrics)
	}
	return providerMetrics
}

func (s *Subscriber) convertAdvancedStatsToMetrics(
	traffic *ioriver.Traffic, providerName string, timestamp int64, advanceMetric ioriver.AdvancedMetric) []metrics.Metrics {
	values := traffic.GetFilteredMetrics(s.serviceId,
		func(metric *ioriver.Metric, metricTimestamp int64) bool {
			if metric.AdvancedMetricName == nil || metric.AdvancedMetricValue == nil {
				return false
			}
			return metric.ProviderName == providerName && metricTimestamp == timestamp && *metric.AdvancedMetricName == advanceMetric.String()
		})

	level.Debug(s.logger).Log("service_id", s.serviceId, "provider", providerName, "time", timestamp, "advanced", advanceMetric, "subscriber", "update")
	providerMetrics := make([]metrics.Metrics, 0, len(values))
	fullProviderName := abbreviationToProviderName(providerName)

	for _, value := range values {
		stat := value.Metrics
		labels := map[string]string{
			"serviceID": s.serviceId, "providerName": fullProviderName,
			"advancedMetricValue": *value.AdvancedMetricValue,
		}
		var advancedMetrics *metrics.MainMetrics
		switch *value.AdvancedMetricName {
		case ioriver.StatusCode.String():
			advancedMetrics = metrics.NewStatusCodeMetrics(labels, timestamp)
		case ioriver.HttpVersion.String():
			advancedMetrics = metrics.NewHttpVersionMetrics(labels, timestamp)
		case ioriver.HttpMethod.String():
			advancedMetrics = metrics.NewHttpMethodMetrics(labels, timestamp)
		default:
			panic("unexpected advanced metric name, must never happen")
		}
		advancedMetrics.Hits.Value = float64(stat.Hits)
		advancedMetrics.Bytes.Value = float64(stat.Bytes)
		providerMetrics = append(providerMetrics, advancedMetrics)
	}
	return providerMetrics
}

func (s *Subscriber) getMaxTimestamp(serviceId string, stats []ioriver.ServiceStats) int64 {
	if len(stats) == 0 {
		level.Debug(s.logger).Log("subscriber", fmt.Sprintf("empty service stat for service %s", s.serviceId))
		return 0
	}

	var timestamps []int64
	for _, stat := range stats {
		if stat.ServiceId == serviceId {
			for _, p := range stat.Points {
				timestamps = append(timestamps, p.Timestamp)
			}
		}
	}
	if len(timestamps) > 0 {
		return slices.Max(timestamps)
	}
	return 0
}

func (s *Subscriber) getTimeRange() (from time.Time, to time.Time) {
	to = time.Now().Add(-s.trafficDelay)
	from = to.Add(metricsGranularity)
	return
}

func getAllProviderNames(stats []ioriver.ServiceStats, serviceId string) []string {
	providerNames := []string{}

	for _, stat := range stats {
		if stat.ServiceId == serviceId {
			for _, point := range stat.Points {
				for _, metrics := range point.Metrics {
					if !slices.Contains(providerNames, metrics.ProviderName) {
						providerNames = append(providerNames, metrics.ProviderName)
					}
				}
			}
		}
	}
	return providerNames
}

func abbreviationToProviderName(name string) string {
	mapping := map[string]string{
		"fs":    "Fastly",
		"cf":    "Cloudflare",
		"cfrnt": "CloudFront",
		"azcdn": "Azure CDN",
		"vcdn":  "vCDN",
	}
	v, ok := mapping[name]
	if ok {
		return v
	}
	return name
}
