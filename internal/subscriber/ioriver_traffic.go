package subscriber

import (
	"fmt"
	"ioriver_exporter/api"
	"ioriver_exporter/internal/metrics"
	"slices"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/ioriver/ioriver-go"
)

// Some CDN providers deliver completed traffic logs with a delay of up to half an hour.
// This const defines how many minutes back we want to retrieve statistics from.
const metricsLookBack = -40 * time.Minute

type IORiverTraffic struct {
	iorClient        api.IORiverClient
	logger           log.Logger
	trafficTimestamp bool
	lastEmittedTS    map[string]int64
}

func NewIORiverTraffic(iorClient api.IORiverClient, logger log.Logger, trafficTimestamp bool) *IORiverTraffic {
	return &IORiverTraffic{
		iorClient:        iorClient,
		logger:           logger,
		trafficTimestamp: trafficTimestamp,
		lastEmittedTS:    make(map[string]int64),
	}
}

// GetTrafficMetrics returns core and advanced (status code / HTTP version / method) traffic metrics for all services.
func (t *IORiverTraffic) GetTrafficMetrics(services []api.ServiceInfo) []metrics.Metrics {
	if len(services) == 0 {
		return []metrics.Metrics{}
	}

	traffic, err := t.getRecentTraffic(services)
	if err != nil {
		level.Warn(t.logger).Log("subscriber", fmt.Sprintf("failed to get recent traffic: %s", err))
		return []metrics.Metrics{}
	}

	allMetrics := make([]metrics.Metrics, 0, len(services)*4)

	for _, service := range services {
		serviceMetrics := t.findTrafficMetrics(traffic, service)
		allMetrics = append(allMetrics, serviceMetrics...)

		serviceMetrics = t.findAdvancedTrafficMetrics(traffic, service, ioriver.StatusCode)
		allMetrics = append(allMetrics, serviceMetrics...)

		serviceMetrics = t.findAdvancedTrafficMetrics(traffic, service, ioriver.HttpVersion)
		allMetrics = append(allMetrics, serviceMetrics...)

		serviceMetrics = t.findAdvancedTrafficMetrics(traffic, service, ioriver.HttpMethod)
		allMetrics = append(allMetrics, serviceMetrics...)
	}

	return allMetrics
}

func (t *IORiverTraffic) getRecentTraffic(services []api.ServiceInfo) (*ioriver.Traffic, error) {
	serviceIDs := make([]string, 0, len(services))
	for _, service := range services {
		serviceIDs = append(serviceIDs, service.Id)
	}

	from, to := t.getTimeRange()
	advancedMetrics := []ioriver.AdvancedMetric{ioriver.StatusCode, ioriver.HttpVersion, ioriver.HttpMethod}
	traffic, err := t.iorClient.RetrieveTrafficOvertime(serviceIDs, from.UnixMilli(), to.UnixMilli(), ioriver.Minute, advancedMetrics)
	if err != nil {
		level.Warn(t.logger).Log("subscriber", fmt.Sprintf("failed to get traffic for services %v: %s", serviceIDs, err))
		return nil, err
	}
	return traffic, nil
}

func (t *IORiverTraffic) findTrafficMetrics(traffic *ioriver.Traffic, service api.ServiceInfo) []metrics.Metrics {
	providerNames := getAllProviderNames(traffic.ServiceStats, service.Id)
	if len(providerNames) == 0 {
		level.Debug(t.logger).Log("subscriber", "no_data", "service_id", service.Id, "reason", "no_providers")
		return []metrics.Metrics{}
	}

	// convert all stats metrics using a per-provider cursor timestamp
	var out []metrics.Metrics
	for _, providerName := range providerNames {
		timestamp := t.selectTimestamp(service.Id, traffic.ServiceStats, "", providerName)
		if timestamp == 0 {
			level.Debug(t.logger).Log("subscriber", "no_data", "service_id", service.Id, "provider", providerName, "reason", "no_timestamp")
			continue
		}
		providerMetrics := t.convertStatsToMetrics(traffic, service.Id, service.Name, providerName, timestamp)
		out = append(out, providerMetrics...)
	}
	return out
}

func (t *IORiverTraffic) findAdvancedTrafficMetrics(traffic *ioriver.Traffic, service api.ServiceInfo, advancedMetric ioriver.AdvancedMetric) []metrics.Metrics {
	providerNames := getAllProviderNames(traffic.ServiceStats, service.Id)
	if len(providerNames) == 0 {
		level.Debug(t.logger).Log("subscriber", "no_data", "service_id", service.Id, "advanced_metric", advancedMetric, "reason", "no_providers")
		return []metrics.Metrics{}
	}

	// convert all advanced stats metrics using a per-provider cursor timestamp
	var out []metrics.Metrics
	for _, providerName := range providerNames {
		timestamp := t.selectTimestamp(service.Id, traffic.ServiceStats, advancedMetric.String(), providerName)
		if timestamp == 0 {
			level.Debug(t.logger).Log("subscriber", "no_data", "service_id", service.Id, "provider", providerName, "advanced_metric", advancedMetric, "reason", "no_timestamp")
			continue
		}
		providerMetrics := t.convertAdvancedStatsToMetrics(traffic, service.Id, service.Name, providerName, timestamp, advancedMetric)
		out = append(out, providerMetrics...)
	}
	return out
}

func (t *IORiverTraffic) convertStatsToMetrics(traffic *ioriver.Traffic, serviceId, serviceName, providerName string, timestamp int64) []metrics.Metrics {
	values := traffic.GetFilteredMetrics(serviceId, func(metric *ioriver.Metric, metricTimestamp int64) bool {
		return metric.ProviderName == providerName && metricTimestamp == timestamp && metric.AdvancedMetricName == nil
	})

	labels := map[string]string{"serviceID": serviceId, "serviceName": serviceName, "providerName": abbreviationToProviderName(providerName)}
	level.Debug(t.logger).Log("service_id", serviceId, "provider", providerName, "time", timestamp, "subscriber", "update")
	providerMetrics := make([]metrics.Metrics, 0, len(values))

	for _, value := range values {
		stat := value.Metrics
		all := metrics.NewAllMetrics(labels, timestamp)
		all.Hits.Value = float64(stat.Hits)
		all.Bytes.Value = float64(stat.Bytes)
		all.CachedHitsPercentage.Value = stat.CachedHitsPercentage
		all.CachedBytesPercentage.Value = stat.CachedBytesPercentage
		all.ErrorsPercentage.Value = stat.ErrorsPercentage
		all.OriginHits.Value = float64(stat.OriginHits)
		all.OriginBytes.Value = float64(stat.OriginBytes)

		providerMetrics = append(providerMetrics, all)
	}
	return providerMetrics
}

func (t *IORiverTraffic) convertAdvancedStatsToMetrics(
	traffic *ioriver.Traffic,
	serviceId,
	serviceName,
	providerName string,
	timestamp int64,
	advancedMetric ioriver.AdvancedMetric,
) []metrics.Metrics {
	values := traffic.GetFilteredMetrics(serviceId,
		func(metric *ioriver.Metric, metricTimestamp int64) bool {
			if metric.AdvancedMetricName == nil || metric.AdvancedMetricValue == nil {
				return false
			}
			return metric.ProviderName == providerName && metricTimestamp == timestamp && *metric.AdvancedMetricName == advancedMetric.String()
		})

	level.Debug(t.logger).Log("service_id", serviceId, "provider", providerName, "time", timestamp, "advanced", advancedMetric, "subscriber", "update")
	providerMetrics := make([]metrics.Metrics, 0, len(values))
	fullProviderName := abbreviationToProviderName(providerName)

	for _, value := range values {
		stat := value.Metrics
		labels := map[string]string{
			"serviceID": serviceId, "serviceName": serviceName, "providerName": fullProviderName,
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
			level.Warn(t.logger).Log(
				"subscriber", "unexpected advanced metric name",
				"service_id", serviceId,
				"advanced_metric_name", *value.AdvancedMetricName,
			)
			continue
		}
		advancedMetrics.Hits.Value = float64(stat.Hits)
		advancedMetrics.Bytes.Value = float64(stat.Bytes)
		providerMetrics = append(providerMetrics, advancedMetrics)
	}
	return providerMetrics
}

// tsKey returns a map key for the per-provider timestamp cursor.
func tsKey(serviceId, providerName, metricType string) string {
	return serviceId + "\x00" + providerName + "\x00" + metricType
}

// collectTimestampsForProvider returns all timestamps available in stats for the
// given service, provider, and metric type.
func (t *IORiverTraffic) collectTimestampsForProvider(serviceId string, stats []ioriver.ServiceStats, advancedMetricName string, providerName string) []int64 {
	var timestamps []int64
	for _, stat := range stats {
		if stat.ServiceId != serviceId {
			continue
		}
		for _, p := range stat.Points {
			for _, metric := range p.Metrics {
				if metric.ProviderName != providerName {
					continue
				}
				if advancedMetricName == "" {
					if metric.AdvancedMetricName == nil {
						timestamps = append(timestamps, p.Timestamp)
						break
					}
				} else if metric.AdvancedMetricName != nil && *metric.AdvancedMetricName == advancedMetricName {
					timestamps = append(timestamps, p.Timestamp)
					break
				}
			}
		}
	}
	return timestamps
}

// selectTimestamp picks which timestamp to emit for a given provider.
// Without trafficTimestamp: always the latest (existing behaviour).
// With trafficTimestamp: the earliest timestamp after the last emitted one,
// so every available minute is reported in order across successive polls.
func (t *IORiverTraffic) selectTimestamp(serviceId string, stats []ioriver.ServiceStats, advancedMetricName, providerName string) int64 {
	timestamps := t.collectTimestampsForProvider(serviceId, stats, advancedMetricName, providerName)
	if len(timestamps) == 0 {
		return 0
	}
	if !t.trafficTimestamp {
		return slices.Max(timestamps)
	}
	key := tsKey(serviceId, providerName, advancedMetricName)
	last := t.lastEmittedTS[key]
	if last == 0 {
		// First emission: start from the most recent data point to match the
		// pre-cursor behaviour and avoid writing out-of-order samples to Prometheus.
		next := slices.Max(timestamps)
		t.lastEmittedTS[key] = next
		return next
	}
	var candidates []int64
	for _, ts := range timestamps {
		if ts > last {
			candidates = append(candidates, ts)
		}
	}
	if len(candidates) == 0 {
		return 0
	}
	next := slices.Min(candidates)
	t.lastEmittedTS[key] = next
	return next
}

func (t *IORiverTraffic) getTimeRange() (from time.Time, to time.Time) {
	to = time.Now()
	from = to.Add(metricsLookBack)
	return
}

func getAllProviderNames(stats []ioriver.ServiceStats, serviceId string) []string {
	providerNames := []string{}

	for _, stat := range stats {
		if stat.ServiceId == serviceId {
			for _, point := range stat.Points {
				for _, m := range point.Metrics {
					if !slices.Contains(providerNames, m.ProviderName) {
						providerNames = append(providerNames, m.ProviderName)
					}
				}
			}
		}
	}
	return providerNames
}

func abbreviationToProviderName(name string) string {
	mapping := map[string]string{
		"fs":         "Fastly",
		"cf":         "Cloudflare",
		"cfrnt":      "CloudFront",
		"azcdn":      "Azure CDN",
		"vcdn":       "vCDN",
		"akamai":     "Akamai",
		"cdnetworks": "CDNetworks",
	}
	v, ok := mapping[name]
	if ok {
		return v
	}
	return name
}
