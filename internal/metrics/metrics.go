package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	Namespace string = "ioriver"
	Subsystem string = "traffic"
)

type Metric struct {
	Name  string
	Help  string
	Value float64
}

type Metrics interface {

	// ToPrometheusMetrics converts to Prometheus metrics.
	ToPrometheusMetrics() []*prometheus.Metric
	// GetTimestamp returns the time when the metrics were collected by IORiver
	GetTimestamp() int64
}

type MainMetrics struct {
	Hits  *Metric
	Bytes *Metric

	labels    map[string]string
	timestamp int64
}

type AllMetrics struct {
	MainMetrics
	CachedHitsPercentage      *Metric
	CachedBytesPercentage     *Metric
	EdgeCachedHitsPercentage  *Metric
	EdgeCachedBytesPercentage *Metric
	ErrorsPercentage          *Metric
}

func NewAllMetrics(labels map[string]string, timestamp int64) *AllMetrics {

	m := &AllMetrics{
		MainMetrics: MainMetrics{
			Hits:      &Metric{Name: "hits", Help: "Total hits served."},
			Bytes:     &Metric{Name: "bytes", Help: "Total bytes served."},
			labels:    labels,
			timestamp: timestamp,
		},
		CachedHitsPercentage:  &Metric{Name: "cached_hits_percentage", Help: "Cached hits percentage."},
		CachedBytesPercentage: &Metric{Name: "cached_bytes_percentage", Help: "Cached bytes percentage."},
		ErrorsPercentage:      &Metric{Name: "errors_percentage", Help: "Error percentage."},
	}
	return m
}

func NewStatusCodeMetrics(labels map[string]string, timestamp int64) *MainMetrics {
	return &MainMetrics{
		Hits:  &Metric{Name: "hits_by_status_code", Help: "Total hits served by status code"},
		Bytes: &Metric{Name: "bytes_by_status_code", Help: "Total bytes served by status code"},

		labels:    labels,
		timestamp: timestamp,
	}
}

func NewHttpVersionMetrics(labels map[string]string, timestamp int64) *MainMetrics {
	return &MainMetrics{
		Hits:  &Metric{Name: "hits_by_http_version", Help: "Total hits served by HTTP version"},
		Bytes: &Metric{Name: "bytes_by_http_version", Help: "Total bytes served by HTTP version"},

		labels:    labels,
		timestamp: timestamp,
	}
}

func NewHttpMethodMetrics(labels map[string]string, timestamp int64) *MainMetrics {
	return &MainMetrics{
		Hits:  &Metric{Name: "hits_by_http_method", Help: "Total hits served by HTTP method"},
		Bytes: &Metric{Name: "bytes_by_http_method", Help: "Total bytes served by HTTP method"},

		labels:    labels,
		timestamp: timestamp,
	}
}

func (m *MainMetrics) ToPrometheusMetrics() []*prometheus.Metric {
	labelNames, labelValues := m.collectLabels()

	metrics := []*Metric{m.Hits, m.Bytes}
	promMetrics := m.toPrometheusMetrics(metrics, labelNames, labelValues)
	return promMetrics
}

func (m *MainMetrics) collectLabels() ([]string, []string) {
	var labelNames, labelValues []string
	for k, v := range m.labels {
		labelNames = append(labelNames, k)
		labelValues = append(labelValues, v)
	}
	return labelNames, labelValues
}

func (*MainMetrics) toPrometheusMetrics(metrics []*Metric, labelNames []string, labelValues []string) []*prometheus.Metric {
	promMetrics := make([]*prometheus.Metric, 0, len(metrics))

	for _, met := range metrics {
		name := prometheus.BuildFQName(Namespace, Subsystem, met.Name)
		desc := prometheus.NewDesc(name, met.Help, labelNames, nil)
		m := prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(met.Value), labelValues...)
		promMetrics = append(promMetrics, &m)
	}
	return promMetrics
}

func (m *MainMetrics) GetTimestamp() int64 {
	return m.timestamp
}

func (m *AllMetrics) ToPrometheusMetrics() []*prometheus.Metric {
	labelNames, labelValues := m.collectLabels()

	metrics := []*Metric{m.Hits, m.Bytes, m.CachedHitsPercentage, m.CachedBytesPercentage, m.ErrorsPercentage}
	promMetrics := m.toPrometheusMetrics(metrics, labelNames, labelValues)
	return promMetrics
}
