package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestMetrics(t *testing.T) {
	metrics := NewMetrics(map[string]string{"a": "b"}, 0)

	if metrics == nil {
		t.Error("failed to create metrics")
	}
}

func TestToPrometheusMetrics(t *testing.T) {
	labels := map[string]string{
		"service": "test-service",
		"region":  "us-east-1",
	}
	timestamp := int64(1234567890)

	m := NewMetrics(labels, timestamp)

	// Set some test values
	m.Hits.Value = 1000
	m.Bytes.Value = 5000000
	m.CachedHitsPercentage.Value = 85.5
	m.CachedBytesPercentage.Value = 90.2
	m.ErrorsPercentage.Value = 0.5

	promMetrics := m.ToPrometheusMetrics()

	// Verify the number of metrics
	if len(promMetrics) != 5 {
		t.Errorf("expected 5 metrics, got %d", len(promMetrics))
	}

	// Helper function to extract metric details
	extractMetricDetails := func(pm *prometheus.Metric) (string, float64, map[string]string) {
		metricDto := &dto.Metric{}
		if err := (*pm).Write(metricDto); err != nil {
			t.Fatalf("failed to write metric: %v", err)
		}

		desc := (*pm).Desc()
		name := desc.String()

		value := 0.0
		if metricDto.Gauge != nil {
			value = metricDto.Gauge.GetValue()
		}

		labels := make(map[string]string)
		for _, label := range metricDto.Label {
			labels[label.GetName()] = label.GetValue()
		}

		return name, value, labels
	}

	// Verify each metric
	expectedMetrics := map[string]float64{
		"ioriver_traffic_hits":                    1000,
		"ioriver_traffic_bytes":                   5000000,
		"ioriver_traffic_cached_hits_percentage":  85.5,
		"ioriver_traffic_cached_bytes_percentage": 90.2,
		"ioriver_traffic_errors_percentage":       0.5,
	}

	foundMetrics := make(map[string]bool)

	for _, pm := range promMetrics {
		name, value, metricLabels := extractMetricDetails(pm)

		// Check if metric name contains expected substring
		var matchedKey string
		for expectedName := range expectedMetrics {
			if containsMetricName(name, expectedName) {
				matchedKey = expectedName
				foundMetrics[expectedName] = true
				break
			}
		}

		if matchedKey == "" {
			t.Errorf("unexpected metric: %s", name)
			continue
		}

		// Verify value
		expectedValue := expectedMetrics[matchedKey]
		if value != expectedValue {
			t.Errorf("metric %s: expected value %f, got %f", matchedKey, expectedValue, value)
		}

		// Verify labels
		if len(metricLabels) != len(labels) {
			t.Errorf("metric %s: expected %d labels, got %d", matchedKey, len(labels), len(metricLabels))
		}

		for k, v := range labels {
			if metricLabels[k] != v {
				t.Errorf("metric %s: expected label %s=%s, got %s", matchedKey, k, v, metricLabels[k])
			}
		}
	}

	// Verify all expected metrics were found
	for expectedName := range expectedMetrics {
		if !foundMetrics[expectedName] {
			t.Errorf("expected metric %s not found", expectedName)
		}
	}
}

// Helper function to check if desc contains the metric name
func containsMetricName(desc, name string) bool {
	// The desc contains the full metric description including name
	// We need to check if it contains the expected metric name
	return len(desc) > 0 && len(name) > 0 && contains(desc, name)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || indexOfSubstring(s, substr) >= 0)
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
