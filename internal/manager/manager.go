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
	RegisterMetricsProvider(provider collectors.MetricsProvider)
	UnregisterMetricsProvider()
}

// SubscriptionManager creates and interrupts subscriptions for IORiver service metrics.
type SubscriptionManager struct {
	serviceIdProvider ServiceIdProvider
	iorClient         api.IORiverClient
	registry          MetricProviderRegistry
	settings          *exporter_settings.Settings
	logger            log.Logger

	mtx        sync.RWMutex
	subscriber *subscriber.Subscriber
	irq        *interrupt
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
func (m *SubscriptionManager) Refresh() {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.subscriber == nil {
		level.Warn(m.logger).Log("subscriber", "not started yet, skipping refresh")
		return
	}

	services := m.serviceIdProvider.GetServicesInfo()
	m.subscriber.UpdateServices(services)
}

// StartSubscription initializes the shared subscriber for all services
// and starts the subscription in a separate goroutine.
func (m *SubscriptionManager) StartSubscription() {
	if m.subscriber != nil {
		level.Warn(m.logger).Log("subscriber", "already started, skipping")
		return
	}

	level.Info(m.logger).Log("subscriber", "start")
	var (
		traffic     = subscriber.NewIORiverTraffic(m.iorClient, m.logger)
		subscriber  = subscriber.NewSubscriber(traffic)
		ctx, cancel = context.WithCancel(context.Background())
		done        = make(chan error, 1)
	)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	m.subscriber = subscriber
	services := m.serviceIdProvider.GetServicesInfo()
	m.subscriber.UpdateServices(services) // initialize the subscriber with the current list of services
	m.registry.RegisterMetricsProvider(subscriber)
	go func() { done <- fmt.Errorf("subscriber: %w", subscriber.Subscribe(ctx)) }()
	m.irq = &interrupt{cancel, done}
}

// Gracefully stop the subscription.
func (m *SubscriptionManager) StopSubscription() {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.irq == nil {
		return
	}

	level.Info(m.logger).Log("subscriber", "stop")
	m.irq.cancel()
	err := <-m.irq.done
	level.Debug(m.logger).Log("interrupt", err)
	m.registry.UnregisterMetricsProvider()
	m.irq = nil
	m.subscriber = nil
}
