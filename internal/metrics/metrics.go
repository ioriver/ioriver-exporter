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

type Metrics struct {
	Hits                      *Metric
	Bytes                     *Metric
	CachedHitsPercentage      *Metric
	CachedBytesPercentage     *Metric
	EdgeCachedHitsPercentage  *Metric
	EdgeCachedBytesPercentage *Metric
	ErrorsPercentage          *Metric
	NumMinutes                *Metric
	MidgressBytes             *Metric
	MidgressHits              *Metric
	OriginHits                *Metric
	OriginBytes               *Metric

	labels    map[string]string
	timestamp int64
}

func NewMetrics(labels map[string]string, timestamp int64) *Metrics {

	m := &Metrics{
		Hits:                  &Metric{Name: "hits", Help: "Total hits served."},
		Bytes:                 &Metric{Name: "bytes", Help: "Total bytes served."},
		CachedHitsPercentage:  &Metric{Name: "cached_hits_percentage", Help: "Cached hits percentage."},
		CachedBytesPercentage: &Metric{Name: "cached_bytes_percentage", Help: "Cached bytes percentage."},
		ErrorsPercentage:      &Metric{Name: "errors_percentage", Help: "Error percentage."},
	}

	m.labels = labels
	m.timestamp = timestamp
	return m
}

// ToPrometheusMetrics converts to Prometheus metrics.
func (m *Metrics) ToPrometheusMetrics() []*prometheus.Metric {
	var labelNames, labelValues []string
	for k, v := range m.labels {
		labelNames = append(labelNames, k)
		labelValues = append(labelValues, v)
	}

	metrics := []*Metric{
		m.Hits,
		m.Bytes,
		m.CachedHitsPercentage,
		m.CachedBytesPercentage,
		m.ErrorsPercentage}
	promMetrics := make([]*prometheus.Metric, 0, len(metrics))

	for _, met := range metrics {
		name := prometheus.BuildFQName(Namespace, Subsystem, met.Name)
		desc := prometheus.NewDesc(name, met.Help, labelNames, nil)
		m := prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(met.Value), labelValues...)
		promMetrics = append(promMetrics, &m)
	}
	return promMetrics
}

// GetTimestamp returns the time when the metrics were collected by IORiver
func (m *Metrics) GetTimestamp() int64 {
	return m.timestamp
}
