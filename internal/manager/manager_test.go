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
		manager      = NewSubscriptionManager(serviceCache, iorClient, registry, settings, level.NewFilter(logger, level.AllowInfo()))
	)

	manager.Refresh()
	checkNbrManagedServices(t, 1, manager)

	serviceCache.services = []ioriver.Service{{Id: "a", Name: "name_a"}, {Id: "b", Name: "name_b"}}
	manager.Refresh()
	checkNbrManagedServices(t, 2, manager)

	serviceCache.services = []ioriver.Service{}
	manager.Refresh()
	checkNbrManagedServices(t, 0, manager)

	exp := []string{
		`level=debug collector="register metrics provider for a"`,
		`level=debug collector="register metrics provider for b"`,
		`level=debug collector="unregister metrics provider for a"`,
		`level=debug collector="unregister metrics provider for b"`,

		`level=info service_id=a service_name=name_a subscriber=start`,

		`level=info service_id=b service_name=name_b subscriber=start`,

		`level=info service_id=a service_name=name_a subscriber=stop`,
		`level=info service_id=b service_name=name_b subscriber=stop`,
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
		manager      = NewSubscriptionManager(serviceCache, iorClient, registry, settings, level.NewFilter(logger, level.AllowInfo()))
	)
	manager.Refresh()
	checkNbrManagedServices(t, 2, manager)

	manager.StopAll()
	checkNbrManagedServices(t, 0, manager)

	exp := []string{
		`level=debug collector="register metrics provider for a"`,
		`level=debug collector="register metrics provider for b"`,
		`level=debug collector="unregister metrics provider for a"`,
		`level=debug collector="unregister metrics provider for b"`,
		`level=info service_id=a service_name=name_a subscriber=start`,
		`level=info service_id=b service_name=name_b subscriber=start`,

		`level=info service_id=a service_name=name_a subscriber=stop`,
		`level=info service_id=b service_name=name_b subscriber=stop`,
	}
	act := strings.Split(strings.TrimSpace(loggerBuffer.String()), "\n")
	tests.AssertStringSliceEqual(t, exp, act)
}

func checkNbrManagedServices(t *testing.T, exp int, manager *SubscriptionManager) {
	t.Helper()
	if len(manager.managed) != exp {
		t.Errorf("unexpected number of manages services %d", len(manager.managed))
	}
}
