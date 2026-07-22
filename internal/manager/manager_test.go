package manager

import (
	"bytes"
	"ioriver_exporter/api"
	"ioriver_exporter/internal/collectors"
	"ioriver_exporter/internal/settings"
	"ioriver_exporter/tests"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	ioriver "github.com/ioriver/ioriver-go"
)

type FakeServiceCache struct {
	services []ioriver.Service
}

func (c *FakeServiceCache) GetServicesInfo() (info []api.ServiceInfo) {
	info = make([]api.ServiceInfo, 0, len(c.services))
	for _, s := range c.services {
		info = append(info, api.ServiceInfo{Id: s.Id, Name: s.Name})
	}
	return info
}
func TestManager(t *testing.T) {
	var (
		serviceCache = &FakeServiceCache{services: []ioriver.Service{{Id: "a", Name: "name_a"}}}
		iorClient    = &tests.FakeIorClient{}
		loggerBuffer = &bytes.Buffer{}
		logger       = log.NewLogfmtLogger(loggerBuffer)
		registry     = collectors.NewTrafficCollector(false, logger)
		settings     = &settings.Settings{}
	)
	var (
		manager = NewSubscriptionManager(serviceCache, iorClient, registry, settings, level.NewFilter(logger, level.AllowInfo()))
	)

	manager.StartSubscription()
	checkSharedSubscriberRunning(t, manager, true)

	serviceCache.services = []ioriver.Service{{Id: "a", Name: "name_a"}, {Id: "b", Name: "name_b"}}
	manager.Refresh()
	checkSharedSubscriberRunning(t, manager, true)

	serviceCache.services = []ioriver.Service{}
	manager.Refresh()
	checkSharedSubscriberRunning(t, manager, true)

	manager.StopSubscription()
	checkSharedSubscriberRunning(t, manager, false)

	exp := []string{
		`level=info subscriber=start`,
		`level=debug collector="register metrics provider"`,
		`level=info subscriber=stop`,
		`level=debug collector="unregister metrics provider"`,
	}
	act := strings.Split(strings.TrimSpace(loggerBuffer.String()), "\n")
	tests.AssertStringSliceEqual(t, exp, act)
}

func TestManagerStopAll(t *testing.T) {
	var (
		serviceCache = &FakeServiceCache{services: []ioriver.Service{{Id: "a", Name: "name_a"}, {Id: "b", Name: "name_b"}}}
		iorClient    = &tests.FakeIorClient{}
		loggerBuffer = &bytes.Buffer{}
		logger       = log.NewLogfmtLogger(loggerBuffer)
		registry     = collectors.NewTrafficCollector(false, logger)
		settings     = &settings.Settings{}
	)
	var (
		manager = NewSubscriptionManager(serviceCache, iorClient, registry, settings, level.NewFilter(logger, level.AllowInfo()))
	)
	manager.StartSubscription()
	checkSharedSubscriberRunning(t, manager, true)

	manager.StopSubscription()
	checkSharedSubscriberRunning(t, manager, false)

	exp := []string{
		`level=info subscriber=start`,
		`level=debug collector="register metrics provider"`,
		`level=info subscriber=stop`,
		`level=debug collector="unregister metrics provider"`,
	}
	act := strings.Split(strings.TrimSpace(loggerBuffer.String()), "\n")
	tests.AssertStringSliceEqual(t, exp, act)
}

func checkSharedSubscriberRunning(t *testing.T, manager *SubscriptionManager, expected bool) {
	t.Helper()
	running := manager.subscriber != nil && manager.irq != nil
	if running != expected {
		t.Errorf("unexpected shared subscriber running state: expected %v got %v", expected, running)
	}
}
