package cache

import (
	"fmt"
	"ioriver_exporter/api"
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
	logger    log.Logger
	mtx       sync.RWMutex
}

func NewServiceCache(iorClient api.IORiverClient, logger log.Logger) *ServiceCache {
	c := &ServiceCache{
		logger:    logger,
		iorClient: iorClient,
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
		level.Debug(c.logger).Log("service_id", s.Id, "cache", "accepted")
		newServices[s.Id] = s
	}
	c.mtx.Lock()
	c.services = newServices
	c.mtx.Unlock()

	return nil
}

// Get a view of the all services in the cache
func (c *ServiceCache) GetServicesInfo() (info []api.ServiceInfo) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	info = make([]api.ServiceInfo, 0, len(c.services))
	for _, s := range c.services {
		info = append(info, api.ServiceInfo{Id: s.Id, Name: s.Name})
	}
	return info
}
