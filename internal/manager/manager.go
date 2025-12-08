package manager

import (
	"context"
	"fmt"
	"ioriver_exporter/api"
	"ioriver_exporter/internal/collectors"
	"ioriver_exporter/internal/subscriber"
	"sync"

	exporter_settings "ioriver_exporter/internal/settings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// ServiceIdProvider is a contract for a provider of the IORiver services information.
type ServiceIdProvider interface {
	GetServicesInfo() []api.ServiceInfo
}

// MetricProviderRegistry is a contract for registering and unregistering metrics providers
// for IORiver services.
type MetricProviderRegistry interface {
	RegisterMetricsProvider(serviceId string, provider collectors.MetricsProvider)
	UnregisterMetricsProvider(serviceId string)
}

// managed is a map that links services
// with a struct that can interrupt the service subscription
type managed = map[api.ServiceInfo]interrupt

// SubscriptionManager creates and interrupts subscriptions for IORiver service metrics.
type SubscriptionManager struct {
	serviceIdProvider ServiceIdProvider
	iorClient         api.IORiverClient
	registry          MetricProviderRegistry
	settings          *exporter_settings.Settings
	logger            log.Logger

	mtx     sync.RWMutex
	managed managed
}

// the value type of the managed services map
// that is used to interrupt subscriptions
type interrupt struct {
	cancel func()
	done   <-chan error
}

func NewSubscriptionManager(
	serviceIdProvider ServiceIdProvider,
	iorClient api.IORiverClient,
	registry MetricProviderRegistry,
	settings *exporter_settings.Settings,
	logger log.Logger) *SubscriptionManager {

	m := &SubscriptionManager{
		serviceIdProvider: serviceIdProvider,
		iorClient:         iorClient,
		registry:          registry,
		settings:          settings,
		logger:            logger,
	}
	return m
}

// Refresh refreshes subscriptions based on the list of services in the cache.
// It creates subscriptions for each service and cancels then when the services is not in the cache anymore.
func (m *SubscriptionManager) Refresh() {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	newManaged := managed{}
	for _, service := range m.serviceIdProvider.GetServicesInfo() {
		if irq, ok := m.managed[service]; ok {
			level.Debug(m.logger).Log(toLogKeyVals(service, "manager", "managed")...)
			newManaged[service] = irq
			delete(m.managed, service)
		} else {
			level.Info(m.logger).Log(toLogKeyVals(service, "subscriber", "start")...)
			newManaged[service] = m.spawn(service.Id)
		}
	}

	// stop polling not existing services anymore
	m.stopAll(m.managed)

	m.managed = newManaged
}

// Gracefully stop all managed subscriptions.
func (m *SubscriptionManager) StopAll() {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.stopAll(m.managed)
}

// stop all given managed subscriptions
func (m *SubscriptionManager) stopAll(managed managed) {
	for service, irq := range managed {
		level.Info(m.logger).Log(toLogKeyVals(service, "subscriber", "stop")...)
		irq.cancel()
		err := <-irq.done
		delete(m.managed, service)
		m.registry.UnregisterMetricsProvider(service.Id)
		level.Debug(m.logger).Log(toLogKeyVals(service, "interrupt", err)...)
	}
}

// spawn a subroutine for a new subscriber
func (m *SubscriptionManager) spawn(serviceId string) interrupt {
	var (
		subscriber  = subscriber.NewSubscriber(m.iorClient, serviceId, m.settings.TrafficDelay, m.logger)
		ctx, cancel = context.WithCancel(context.Background())
		done        = make(chan error, 1)
	)
	m.registry.RegisterMetricsProvider(serviceId, subscriber)
	go func() { done <- fmt.Errorf("realtime: %w", subscriber.Subscribe(ctx)) }()

	return interrupt{cancel, done}
}

func toLogKeyVals(service api.ServiceInfo, vals ...any) []any {
	params := []any{"service_id", service.Id, "service_name", service.Name}
	return append(params, vals...)
}
