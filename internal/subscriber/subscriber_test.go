package subscriber

import (
	"bytes"
	"context"
	"fmt"
	"ioriver_exporter/tests"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/ioriver/ioriver-go"
)

func TestSubscriber(t *testing.T) {

	var (
		iorClient    = &tests.FakeIorClient{}
		loggerBuffer = &bytes.Buffer{}
		logger       = log.NewLogfmtLogger(loggerBuffer)
		trafficDelay = 1 * time.Minute
		subscriber   = NewSubscriber(iorClient, tests.ServiceId, trafficDelay, level.NewFilter(logger, level.AllowDebug()))
	)

	startStopSubscription(t, subscriber)

	exp := []string{
		fmt.Sprintf("level=debug service_id=%s provider=cfrnt time=1752827340000 subscriber=update", tests.ServiceId),
		fmt.Sprintf("level=debug service_id=%s provider=fs time=1752827340000 subscriber=update", tests.ServiceId),
		fmt.Sprintf("level=debug service_id=%s provider=cfrnt time=1752827340000 advanced=%s subscriber=update", tests.ServiceId, ioriver.StatusCode),
		fmt.Sprintf("level=debug service_id=%s provider=fs time=1752827340000 advanced=%s subscriber=update", tests.ServiceId, ioriver.StatusCode),
		fmt.Sprintf("level=debug service_id=%s provider=cfrnt time=1752827340000 advanced=%s subscriber=update", tests.ServiceId, ioriver.HttpVersion),
		fmt.Sprintf("level=debug service_id=%s provider=fs time=1752827340000 advanced=%s subscriber=update", tests.ServiceId, ioriver.HttpVersion),
		fmt.Sprintf("level=debug service_id=%s provider=cfrnt time=1752827340000 advanced=%s subscriber=update", tests.ServiceId, ioriver.HttpMethod),
		fmt.Sprintf("level=debug service_id=%s provider=fs time=1752827340000 advanced=%s subscriber=update", tests.ServiceId, ioriver.HttpMethod),
	}
	act := strings.Split(strings.TrimSpace(loggerBuffer.String()), "\n")
	tests.AssertStringSliceEqual(t, exp, act)
}

func TestSubscriberFailedToGetStat(t *testing.T) {
	var (
		iorClient    = &tests.FakeIorClient{TrafficResponseJson: "not_valid"}
		trafficDelay = 1 * time.Minute
		loggerBuffer = &bytes.Buffer{}
		logger       = log.NewLogfmtLogger(loggerBuffer)
		subscriber   = NewSubscriber(iorClient, tests.ServiceId, trafficDelay, level.NewFilter(logger, level.AllowDebug()))
	)

	startStopSubscription(t, subscriber)

	const exp = "level=warn subscriber=\"failed to get traffic for service 15e72be2-cb5a-4451-90a7-73e72553eb2a:"
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
		trafficDelay = 1 * time.Minute
		loggerBuffer = &bytes.Buffer{}
		logger       = log.NewLogfmtLogger(loggerBuffer)
		subscriber   = NewSubscriber(iorClient, tests.ServiceId, trafficDelay, level.NewFilter(logger, level.AllowDebug()))
	)

	startStopSubscription(t, subscriber)

	exp := fmt.Sprintf("level=debug subscriber=\"no statistic points for service %s", tests.ServiceId)
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
		trafficDelay = 1 * time.Minute
		subscriber   = NewSubscriber(iorClient, tests.ServiceId, trafficDelay, level.NewFilter(logger, level.AllowDebug()))
	)

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

	expectedMetricCount := 22
	if len(promMetrics) != expectedMetricCount {
		t.Errorf("expected %d Prometheus metrics, got %d", expectedMetricCount, len(promMetrics))
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
