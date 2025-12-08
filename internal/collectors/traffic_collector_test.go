package collectors

import (
	"bytes"
	"testing"
	"time"

	"ioriver_exporter/internal/metrics"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// FakeMetricsProvider is a mock implementation of MetricsProvider for testing
type FakeMetricsProvider struct {
	metricsToReturn []PrometheusMetric
}

func (f *FakeMetricsProvider) GetPrometheusMetrics() []PrometheusMetric {
	return f.metricsToReturn
}

// Helper function to create a test Prometheus metric
func createTestMetric(name string, value float64, timestamp int64) PrometheusMetric {
	desc := prometheus.NewDesc(
		prometheus.BuildFQName(metrics.Namespace, metrics.Subsystem, name),
		"Test metric",
		[]string{"service"},
		nil,
	)
	metric := prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, "test-service")
	return PrometheusMetric{
		Metric:    &metric,
		Timestamp: timestamp,
	}
}

func TestTrafficCollectorDescribe(t *testing.T) {
	logger := log.NewNopLogger()
	collector := NewTrafficCollector(false, logger)

	descChan := make(chan *prometheus.Desc, 1)
	collector.Describe(descChan)
	close(descChan)

	count := 0
	for desc := range descChan {
		count++
		if desc == nil {
			t.Error("received nil descriptor")
		}
	}

	if count != 1 {
		t.Errorf("expected 1 descriptor, got %d", count)
	}
}

func TestTrafficCollectorRegisterUnregisterMetricsProvider(t *testing.T) {
	logBuffer := &bytes.Buffer{}
	logger := log.NewLogfmtLogger(logBuffer)
	logger = level.NewFilter(logger, level.AllowDebug())
	collector := NewTrafficCollector(false, logger)

	serviceId := "test-service-123"
	provider := &FakeMetricsProvider{}

	// Register first
	collector.RegisterMetricsProvider(serviceId, provider)

	// Verify it exists
	collector.mtx.RLock()
	_, exists := collector.metricsProviders[serviceId]
	collector.mtx.RUnlock()

	if !exists {
		t.Fatal("provider should exist before unregister")
	}

	// Clear log buffer
	logBuffer.Reset()

	// Unregister
	collector.UnregisterMetricsProvider(serviceId)

	// Verify it's removed
	_, exists = collector.metricsProviders[serviceId]

	if exists {
		t.Error("provider should not exist after unregister")
	}
}

func TestTrafficCollectorCollectWithTimestamp(t *testing.T) {
	logger := log.NewNopLogger()
	collector := NewTrafficCollector(true, logger)

	// Create fake metrics with specific timestamp
	timestamp := int64(1234567890000)
	testMetrics := []PrometheusMetric{
		createTestMetric("hits", 100, timestamp),
		createTestMetric("bytes", 5000, timestamp),
	}

	provider := &FakeMetricsProvider{metricsToReturn: testMetrics}
	collector.RegisterMetricsProvider("service-1", provider)

	// Collect metrics
	metricChan := make(chan prometheus.Metric, 10)
	collector.Collect(metricChan)
	close(metricChan)

	// Verify collected metrics
	count := 0
	for metric := range metricChan {
		count++

		// Write metric to DTO to inspect
		metricDto := &dto.Metric{}
		if err := metric.Write(metricDto); err != nil {
			t.Fatalf("failed to write metric: %v", err)
		}

		// When trafficTimestamp is true, timestamp should be set
		if metricDto.TimestampMs == nil {
			t.Error("expected timestamp when trafficTimestamp=true")
		} else if *metricDto.TimestampMs != timestamp {
			t.Errorf("expected timestamp %d, got %d", timestamp, *metricDto.TimestampMs)
		}
	}

	if count != len(testMetrics) {
		t.Errorf("expected %d metrics, got %d", len(testMetrics), count)
	}
}

func TestTrafficCollectorCollectMultipleProviders(t *testing.T) {
	logger := log.NewNopLogger()
	collector := NewTrafficCollector(false, logger)

	// Create multiple providers with different metrics
	timestamp := time.Now().UnixMilli()

	provider1 := &FakeMetricsProvider{
		metricsToReturn: []PrometheusMetric{
			createTestMetric("hits", 100, timestamp),
		},
	}

	provider2 := &FakeMetricsProvider{
		metricsToReturn: []PrometheusMetric{
			createTestMetric("bytes", 5000, timestamp),
			createTestMetric("errors", 10, timestamp),
		},
	}

	collector.RegisterMetricsProvider("service-1", provider1)
	collector.RegisterMetricsProvider("service-2", provider2)

	// Collect metrics
	metricChan := make(chan prometheus.Metric, 10)
	collector.Collect(metricChan)
	close(metricChan)

	// Verify total count
	count := 0
	for range metricChan {
		count++
	}

	expectedCount := len(provider1.metricsToReturn) + len(provider2.metricsToReturn)
	if count != expectedCount {
		t.Errorf("expected %d total metrics, got %d", expectedCount, count)
	}
}
