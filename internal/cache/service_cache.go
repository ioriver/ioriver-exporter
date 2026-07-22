package cache

import (
	"fmt"
	"ioriver_exporter/api"
	"ioriver_exporter/internal/filter"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/ioriver/ioriver-go"
)

// ServiceCache keeps IORiver services available for the provider API token in the map
// and refreshes them periodically.
type ServiceCache struct {
	services  map[string]ioriver.Service
	iorClient api.IORiverClient
	filter    *filter.ServiceFilter
	logger    log.Logger
	mtx       sync.RWMutex
}

func NewServiceCache(iorClient api.IORiverClient, svcFilter *filter.ServiceFilter, logger log.Logger) *ServiceCache {
	c := &ServiceCache{
		logger:    logger,
		iorClient: iorClient,
		filter:    svcFilter,
	}
	if c.filter == nil {
		c.filter, _ = filter.NewServiceFilter(nil, "", "", "")
	}
	return c
}

// Pull the list of services to refresh the cache.
func (c *ServiceCache) Refresh() error {
	services, err := c.iorClient.ListServices()
	if err != nil {
		level.Warn(c.logger).Log("service_cache", fmt.Sprintf("failed to update the service ID list: %s", err))
		return err
	}

	level.Debug(c.logger).Log("service_cache", fmt.Sprintf("fetched %d services", len(services)))

	newServices := map[string]ioriver.Service{}
	for _, s := range services {
		newServices[s.Id] = s
	}
	c.mtx.Lock()
	c.services = newServices
	c.mtx.Unlock()

	return nil
}

// Get a view of the all services in the cache
func (c *ServiceCache) GetServicesInfo() (services []api.ServiceInfo) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	services = make([]api.ServiceInfo, 0, len(c.services))
	for _, s := range c.services {
		services = append(services, api.ServiceInfo{Id: s.Id, Name: s.Name})
	}
	services = c.filter.Apply(services)
	for _, s := range services {
		level.Debug(c.logger).Log("msg", "service in cache after filter", "service_id", s.Id, "service_name", s.Name)
	}
	return services
}
