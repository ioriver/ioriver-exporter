package filter

import (
	"fmt"
	"ioriver_exporter/api"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ServiceFilter applies zero or more filters to a slice of services.
// Filters are applied in order: explicit IDs → allowlist → blocklist → shard.
// A ServiceFilter with no active filters is a no-op.
type ServiceFilter struct {
	ids       map[string]struct{} // explicit service IDs; nil means "accept all"
	allowlist *regexp.Regexp      // keep only services whose name matches; nil means "accept all"
	blocklist *regexp.Regexp      // remove services whose name matches; nil means "remove none"
	shardN    int                 // 1-based shard index (0 means sharding disabled)
	shardM    int                 // total number of shards
}

// NewServiceFilter constructs a ServiceFilter from the raw flag values.
//
//   - ids: service IDs to export (empty slice = export all)
//   - allowlist: regex; only services whose name matches are exported (empty = no filter)
//   - blocklist: regex; services whose name matches are excluded (empty = no filter)
//   - shard: "n/m" string, e.g. "1/3" (empty = no sharding)
func NewServiceFilter(ids []string, allowlist, blocklist, shard string) (*ServiceFilter, error) {
	f := &ServiceFilter{}

	// explicit service IDs
	if len(ids) > 0 {
		f.ids = make(map[string]struct{}, len(ids))
		for _, id := range ids {
			f.ids[id] = struct{}{}
		}
	}

	// allowlist regex
	if allowlist != "" {
		re, err := regexp.Compile(allowlist)
		if err != nil {
			return nil, fmt.Errorf("invalid -service-allowlist regex %q: %w", allowlist, err)
		}
		f.allowlist = re
	}

	// blocklist regex
	if blocklist != "" {
		re, err := regexp.Compile(blocklist)
		if err != nil {
			return nil, fmt.Errorf("invalid -service-blocklist regex %q: %w", blocklist, err)
		}
		f.blocklist = re
	}

	// shard "n/m"
	if shard != "" {
		parts := strings.SplitN(strings.TrimSpace(shard), "/", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid -service-shard %q: expected format n/m (e.g. 1/3)", shard)
		}
		n, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		m, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("invalid -service-shard %q: n and m must be integers", shard)
		}
		if m < 1 {
			return nil, fmt.Errorf("invalid -service-shard %q: m must be >= 1", shard)
		}
		if n < 1 || n > m {
			return nil, fmt.Errorf("invalid -service-shard %q: n must satisfy 1 <= n <= m", shard)
		}
		f.shardN = n
		f.shardM = m
	}

	return f, nil
}

// Apply returns the subset of services that pass all active filters.
func (f *ServiceFilter) Apply(services []api.ServiceInfo) []api.ServiceInfo {
	// 1. Explicit service IDs
	if f.ids != nil {
		filtered := make([]api.ServiceInfo, 0, len(services))
		for _, svc := range services {
			if _, ok := f.ids[svc.Id]; ok {
				filtered = append(filtered, svc)
			}
		}
		services = filtered
	}

	// 2. Allowlist
	if f.allowlist != nil {
		filtered := make([]api.ServiceInfo, 0, len(services))
		for _, svc := range services {
			if f.allowlist.MatchString(svc.Name) {
				filtered = append(filtered, svc)
			}
		}
		services = filtered
	}

	// 3. Blocklist
	if f.blocklist != nil {
		filtered := make([]api.ServiceInfo, 0, len(services))
		for _, svc := range services {
			if !f.blocklist.MatchString(svc.Name) {
				filtered = append(filtered, svc)
			}
		}
		services = filtered
	}

	// 4. Shard — sort by Id alphabetically for deterministic assignment,
	//    keep services at positions where (i % m) == (n-1).
	if f.shardM > 0 {
		sorted := make([]api.ServiceInfo, len(services))
		copy(sorted, services)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Id < sorted[j].Id
		})
		filtered := sorted[:0]
		for i, svc := range sorted {
			if i%f.shardM == f.shardN-1 {
				filtered = append(filtered, svc)
			}
		}
		services = filtered
	}

	return services
}
