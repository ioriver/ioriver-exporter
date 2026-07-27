package subscriber

import (
	"bytes"
	"context"
	"fmt"
	"ioriver_exporter/api"
	"ioriver_exporter/tests"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/ioriver/ioriver-go"
	dto "github.com/prometheus/client_model/go"
)

func TestSubscriber(t *testing.T) {

	var (
		iorClient    = &tests.FakeIorClient{}
		loggerBuffer = &bytes.Buffer{}
		logger       = log.NewLogfmtLogger(loggerBuffer)
		traffic      = NewIORiverTraffic(iorClient, level.NewFilter(logger, level.AllowDebug()), false)
		subscriber   = NewSubscriber(traffic, 60*time.Second)
	)
	subscriber.UpdateServices([]api.ServiceInfo{{Id: tests.ServiceId, Name: "ioriver"}})

	startStopSubscription(t, subscriber)

	exp := []string{
		fmt.Sprintf("level=debug service_id=%s provider=cfrnt time=1752827340000 subscriber=update", tests.ServiceId),
		fmt.Sprintf("level=debug service_id=%s provider=fs time=1752827340000 subscriber=update", tests.ServiceId),
		fmt.Sprintf("level=debug service_id=%s provider=cfrnt time=1752827340000 advanced=%s subscriber=update", tests.ServiceId, ioriver.StatusCode),
		fmt.Sprintf("level=debug subscriber=no_data service_id=%s provider=fs advanced_metric=%s reason=no_timestamp", tests.ServiceId, ioriver.StatusCode),
		fmt.Sprintf("level=debug subscriber=no_data service_id=%s provider=cfrnt advanced_metric=%s reason=no_timestamp", tests.ServiceId, ioriver.HttpVersion),
		fmt.Sprintf("level=debug service_id=%s provider=fs time=1752827340000 advanced=%s subscriber=update", tests.ServiceId, ioriver.HttpVersion),
		fmt.Sprintf("level=debug subscriber=no_data service_id=%s provider=cfrnt advanced_metric=%s reason=no_timestamp", tests.ServiceId, ioriver.HttpMethod),
		fmt.Sprintf("level=debug service_id=%s provider=fs time=1752827340000 advanced=%s subscriber=update", tests.ServiceId, ioriver.HttpMethod),
	}
	act := strings.Split(strings.TrimSpace(loggerBuffer.String()), "\n")
	tests.AssertStringSliceEqual(t, exp, act)
}

func TestSubscriberFailedToGetStat(t *testing.T) {
	var (
		iorClient    = &tests.FakeIorClient{TrafficResponseJson: "not_valid"}
		loggerBuffer = &bytes.Buffer{}
		logger       = log.NewLogfmtLogger(loggerBuffer)
		traffic      = NewIORiverTraffic(iorClient, level.NewFilter(logger, level.AllowDebug()), false)
		subscriber   = NewSubscriber(traffic, 60*time.Second)
	)
	subscriber.UpdateServices([]api.ServiceInfo{{Id: tests.ServiceId, Name: "ioriver"}})

	startStopSubscription(t, subscriber)

	const exp = "level=warn subscriber=\"failed to get traffic for services ["
	act := strings.TrimSpace(loggerBuffer.String())
	if !strings.Contains(act, exp) {
		t.Error("unexpected warning")
	}
}

func TestSubscriberStatHasNoPoints(t *testing.T) {
	const resp = `{
	"serviceStats": [
		{
			"tests.ServiceId": "15e72be2-cb5a-4451-90a7-73e72553eb2a",
			"points": []
		}]}`

	var (
		iorClient    = &tests.FakeIorClient{TrafficResponseJson: resp}
		loggerBuffer = &bytes.Buffer{}
		logger       = log.NewLogfmtLogger(loggerBuffer)
		traffic      = NewIORiverTraffic(iorClient, level.NewFilter(logger, level.AllowDebug()), false)
		subscriber   = NewSubscriber(traffic, 60*time.Second)
	)
	subscriber.UpdateServices([]api.ServiceInfo{{Id: tests.ServiceId, Name: "ioriver"}})

	startStopSubscription(t, subscriber)

	exp := fmt.Sprintf("level=debug subscriber=no_data service_id=%s reason=no_providers", tests.ServiceId)
	act := strings.TrimSpace(loggerBuffer.String())
	if !strings.Contains(act, exp) {
		t.Error("unexpected debug message")
	}
}

func TestGetPrometheusMetrics(t *testing.T) {
	var (
		iorClient    = &tests.FakeIorClient{}
		loggerBuffer = &bytes.Buffer{}
		logger       = log.NewLogfmtLogger(loggerBuffer)
		traffic      = NewIORiverTraffic(iorClient, level.NewFilter(logger, level.AllowDebug()), false)
		subscriber   = NewSubscriber(traffic, 60*time.Second)
	)
	subscriber.UpdateServices([]api.ServiceInfo{{Id: tests.ServiceId, Name: "ioriver"}})

	// Update metrics by starting and stopping subscription
	startStopSubscription(t, subscriber)

	promMetrics := subscriber.GetPrometheusMetrics()

	if len(promMetrics) == 0 {
		t.Fatal("expected Prometheus metrics, got none")
	}

	// Verify expected timestamp (from fake client data)
	expectedTimestamp := int64(1752827340000)
	if promMetrics[0].Timestamp != expectedTimestamp {
		t.Errorf("expected timestamp %d, got %d", expectedTimestamp, promMetrics[0].Timestamp)
	}

	expectedMetricCount := 20
	if len(promMetrics) != expectedMetricCount {
		t.Errorf("expected %d Prometheus metrics, got %d", expectedMetricCount, len(promMetrics))
	}

	// Verify serviceName label is present on ALL metrics, and origin metrics are emitted
	foundOriginHits := false
	foundOriginBytes := false

	for _, pm := range promMetrics {
		metricDto := &dto.Metric{}
		if err := (*pm.Metric).Write(metricDto); err != nil {
			t.Fatalf("failed to write metric: %v", err)
		}
		hasServiceName := false
		for _, label := range metricDto.Label {
			if label.GetName() == "serviceName" && label.GetValue() == "ioriver" {
				hasServiceName = true
			}
		}
		if !hasServiceName {
			descStr := (*pm.Metric).Desc().String()
			t.Errorf("metric %s is missing serviceName=ioriver label", descStr)
		}
		descStr := (*pm.Metric).Desc().String()
		if strings.Contains(descStr, "ioriver_traffic_origin_hits") {
			foundOriginHits = true
		}
		if strings.Contains(descStr, "ioriver_traffic_origin_bytes") {
			foundOriginBytes = true
		}
	}

	if !foundOriginHits {
		t.Error("expected ioriver_traffic_origin_hits metric to be exported")
	}
	if !foundOriginBytes {
		t.Error("expected ioriver_traffic_origin_bytes metric to be exported")
	}
}

func TestSubscriberEmptyServiceListClearsMetrics(t *testing.T) {
	var (
		iorClient  = &tests.FakeIorClient{}
		traffic    = NewIORiverTraffic(iorClient, log.NewNopLogger(), false)
		subscriber = NewSubscriber(traffic, 60*time.Second)
	)

	// Populate metrics with a valid service.
	subscriber.UpdateServices([]api.ServiceInfo{{Id: tests.ServiceId, Name: "ioriver"}})
	startStopSubscription(t, subscriber)

	if len(subscriber.GetPrometheusMetrics()) == 0 {
		t.Fatal("expected metrics after subscribing with a service")
	}

	// Remove all services and run another update cycle.
	subscriber.UpdateServices(nil)
	startStopSubscription(t, subscriber)

	if got := subscriber.GetPrometheusMetrics(); len(got) != 0 {
		t.Errorf("expected stale metrics to be cleared after service list becomes empty, got %d metrics", len(got))
	}
}

func startStopSubscription(t *testing.T, subscriber *Subscriber) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan error, 1)
	go func() { ch <- subscriber.Subscribe(ctx) }()
	cancel()
	<-ch
}

func TestTrafficTimestampCursorAdvancesPerPoll(t *testing.T) {
	const (
		t1 = int64(1752827340000)
		t2 = int64(1752827400000) // t1 + 60s
		t3 = int64(1752827460000) // t1 + 120s
	)

	twoPointResp := fmt.Sprintf(`{
		"serviceStats": [{
			"serviceID": "%s",
			"points": [
				{
					"timestamp": %d,
					"metrics": [{"providerName": "cfrnt", "geo": null, "advancedMetricName": null, "advancedMetricValue": null, "metrics": {"hits": 100, "bytes": 1000, "cachedHitsPercentage": 90.0, "cachedBytesPercentage": 90.0, "errorsPercentage": 1.0}}]
				},
				{
					"timestamp": %d,
					"metrics": [{"providerName": "cfrnt", "geo": null, "advancedMetricName": null, "advancedMetricValue": null, "metrics": {"hits": 200, "bytes": 2000, "cachedHitsPercentage": 90.0, "cachedBytesPercentage": 90.0, "errorsPercentage": 1.0}}]
				}
			]
		}]
	}`, tests.ServiceId, t1, t2)

	iorClient := &tests.FakeIorClient{TrafficResponseJson: twoPointResp}
	traffic := NewIORiverTraffic(iorClient, log.NewNopLogger(), true)
	services := []api.ServiceInfo{{Id: tests.ServiceId, Name: "ioriver"}}

	// Poll 1: cursor=0, expect t2 (MAX — avoids out-of-order writes to Prometheus on restart).
	m1 := traffic.GetTrafficMetrics(services)
	if len(m1) == 0 {
		t.Fatal("poll 1: expected metrics, got none")
	}
	for _, m := range m1 {
		if m.GetTimestamp() != t2 {
			t.Errorf("poll 1: expected timestamp %d, got %d", t2, m.GetTimestamp())
		}
	}

	// Poll 2: cursor=t2, no timestamps > t2 — expect no metrics (caught up).
	m2 := traffic.GetTrafficMetrics(services)
	if len(m2) != 0 {
		t.Errorf("poll 2: expected no metrics when caught up, got %d", len(m2))
	}

	// Add t3 to the response and verify the cursor advances to it.
	threePointResp := fmt.Sprintf(`{
		"serviceStats": [{
			"serviceID": "%s",
			"points": [
				{"timestamp": %d, "metrics": [{"providerName": "cfrnt", "geo": null, "advancedMetricName": null, "advancedMetricValue": null, "metrics": {"hits": 100, "bytes": 1000, "cachedHitsPercentage": 90.0, "cachedBytesPercentage": 90.0, "errorsPercentage": 1.0}}]},
				{"timestamp": %d, "metrics": [{"providerName": "cfrnt", "geo": null, "advancedMetricName": null, "advancedMetricValue": null, "metrics": {"hits": 200, "bytes": 2000, "cachedHitsPercentage": 90.0, "cachedBytesPercentage": 90.0, "errorsPercentage": 1.0}}]},
				{"timestamp": %d, "metrics": [{"providerName": "cfrnt", "geo": null, "advancedMetricName": null, "advancedMetricValue": null, "metrics": {"hits": 300, "bytes": 3000, "cachedHitsPercentage": 90.0, "cachedBytesPercentage": 90.0, "errorsPercentage": 1.0}}]}
			]
		}]
	}`, tests.ServiceId, t1, t2, t3)
	iorClient.TrafficResponseJson = threePointResp

	// Poll 3: cursor=t2, candidates=[t3], expect t3.
	m3 := traffic.GetTrafficMetrics(services)
	if len(m3) == 0 {
		t.Fatal("poll 3: expected metrics for t3, got none")
	}
	for _, m := range m3 {
		if m.GetTimestamp() != t3 {
			t.Errorf("poll 3: expected timestamp %d, got %d", t3, m.GetTimestamp())
		}
	}
}
