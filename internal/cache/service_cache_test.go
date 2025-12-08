package cache

import (
	"bytes"
	"ioriver_exporter/tests"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

func TestServiceCacheRefresh(t *testing.T) {
	var (
		iorClient    = &tests.FakeIorClient{}
		loggerBuffer = &bytes.Buffer{}
		logger       = log.NewLogfmtLogger(loggerBuffer)
		cache        = NewServiceCache(iorClient, level.NewFilter(logger, level.AllowDebug()))
	)

	cache.Refresh()

	const exp = `level=debug service_cache="fetched 1 services"`
	act := strings.TrimSpace(loggerBuffer.String())
	if !strings.Contains(act, exp) {
		t.Error("unexpected debug message")
	}

	serviceInfo := cache.GetServicesInfo()
	if len(serviceInfo) != 1 || serviceInfo[0].Id != tests.ServiceId {
		t.Error("unexpected service info")
	}

}

func TestServiceCacheGetServiceInfo(t *testing.T) {
	var (
		iorClient = &tests.FakeIorClient{}
		cache     = NewServiceCache(iorClient, log.NewNopLogger())
	)

	serviceInfo := cache.GetServicesInfo()

	if len(serviceInfo) != 0 {
		t.Error("unexpected service info length")
	}
}
