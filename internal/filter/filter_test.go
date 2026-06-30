package filter

import (
	"ioriver_exporter/api"
	"reflect"
	"sort"
	"testing"
)

// helpers

func svc(id, name string) api.ServiceInfo { return api.ServiceInfo{Id: id, Name: name} }

func sortedIDs(svcs []api.ServiceInfo) []string {
	ids := make([]string, len(svcs))
	for i, s := range svcs {
		ids[i] = s.Id
	}
	sort.Strings(ids)
	return ids
}

var allServices = []api.ServiceInfo{
	svc("aaa", "Production A"),
	svc("bbb", "Staging B"),
	svc("ccc", "Production C"),
	svc("ddd", "Test D"),
	svc("eee", "Production E"),
}

// ── NewServiceFilter error cases ─────────────────────────────────────────────

func TestNewServiceFilter_invalid(t *testing.T) {
	cases := []struct {
		name      string
		allowlist string
		blocklist string
		shard     string
	}{
		{"bad allowlist regex", "[invalid", "", ""},
		{"bad blocklist regex", "", "[invalid", ""},
		{"shard no slash", "", "", "13"},
		{"shard zero n", "", "", "0/3"},
		{"shard n > m", "", "", "4/3"},
		{"shard zero m", "", "", "1/0"},
		{"shard non-integer", "", "", "abc/2"},
		{"shard non-integer m", "", "", "1/x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewServiceFilter(nil, tc.allowlist, tc.blocklist, tc.shard)
			if err == nil {
				t.Fatalf("expected error for shard=%q allowlist=%q blocklist=%q, got nil", tc.shard, tc.allowlist, tc.blocklist)
			}
		})
	}
}

// ── Apply: no filter ─────────────────────────────────────────────────────────

func TestApply_noFilter(t *testing.T) {
	f, err := NewServiceFilter(nil, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	got := f.Apply(allServices)
	if !reflect.DeepEqual(got, allServices) {
		t.Fatalf("expected all services, got %v", got)
	}
}

// ── Apply: explicit service IDs ──────────────────────────────────────────────

func TestApply_serviceIDs(t *testing.T) {
	cases := []struct {
		name    string
		ids     []string
		wantIDs []string
	}{
		{"single", []string{"aaa"}, []string{"aaa"}},
		{"multiple", []string{"aaa", "ccc"}, []string{"aaa", "ccc"}},
		{"unknown ID silently dropped", []string{"aaa", "zzz"}, []string{"aaa"}},
		{"all unknown", []string{"zzz"}, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := NewServiceFilter(tc.ids, "", "", "")
			if err != nil {
				t.Fatal(err)
			}
			got := f.Apply(allServices)
			gotIDs := sortedIDs(got)
			sort.Strings(tc.wantIDs)
			if !reflect.DeepEqual(gotIDs, tc.wantIDs) {
				t.Fatalf("want %v, got %v", tc.wantIDs, gotIDs)
			}
		})
	}
}

// ── Apply: allowlist ─────────────────────────────────────────────────────────

func TestApply_allowlist(t *testing.T) {
	cases := []struct {
		name      string
		allowlist string
		wantIDs   []string
	}{
		{"prefix Production", `^Production`, []string{"aaa", "ccc", "eee"}},
		{"suffix B", `B$`, []string{"bbb"}},
		{"matches none", `^XYZ`, []string{}},
		{"matches all", `.*`, []string{"aaa", "bbb", "ccc", "ddd", "eee"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := NewServiceFilter(nil, tc.allowlist, "", "")
			if err != nil {
				t.Fatal(err)
			}
			got := f.Apply(allServices)
			gotIDs := sortedIDs(got)
			sort.Strings(tc.wantIDs)
			if !reflect.DeepEqual(gotIDs, tc.wantIDs) {
				t.Fatalf("want %v, got %v", tc.wantIDs, gotIDs)
			}
		})
	}
}

// ── Apply: blocklist ─────────────────────────────────────────────────────────

func TestApply_blocklist(t *testing.T) {
	cases := []struct {
		name      string
		blocklist string
		wantIDs   []string
	}{
		{"exclude Test", `Test`, []string{"aaa", "bbb", "ccc", "eee"}},
		{"exclude Staging", `Staging`, []string{"aaa", "ccc", "ddd", "eee"}},
		{"exclude nothing", `^XYZ`, []string{"aaa", "bbb", "ccc", "ddd", "eee"}},
		{"exclude all", `.*`, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := NewServiceFilter(nil, "", tc.blocklist, "")
			if err != nil {
				t.Fatal(err)
			}
			got := f.Apply(allServices)
			gotIDs := sortedIDs(got)
			sort.Strings(tc.wantIDs)
			if !reflect.DeepEqual(gotIDs, tc.wantIDs) {
				t.Fatalf("want %v, got %v", tc.wantIDs, gotIDs)
			}
		})
	}
}

// ── Apply: shard ─────────────────────────────────────────────────────────────

func TestApply_shard(t *testing.T) {
	// allServices sorted by Id: aaa bbb ccc ddd eee
	// shard 1/3 → indices 0,3 → aaa, ddd
	// shard 2/3 → indices 1,4 → bbb, eee
	// shard 3/3 → indices 2   → ccc
	// shard 1/1 → all
	cases := []struct {
		shard   string
		wantIDs []string
	}{
		{"1/3", []string{"aaa", "ddd"}},
		{"2/3", []string{"bbb", "eee"}},
		{"3/3", []string{"ccc"}},
		{"1/1", []string{"aaa", "bbb", "ccc", "ddd", "eee"}},
		{"1/5", []string{"aaa"}},
		{"5/5", []string{"eee"}},
		{" 1/3 ", []string{"aaa", "ddd"}},  // whitespace-padded (common from env vars)
		{"1 / 3", []string{"aaa", "ddd"}},  // spaces around slash
	}
	for _, tc := range cases {
		t.Run(tc.shard, func(t *testing.T) {
			f, err := NewServiceFilter(nil, "", "", tc.shard)
			if err != nil {
				t.Fatal(err)
			}
			got := f.Apply(allServices)
			gotIDs := sortedIDs(got)
			sort.Strings(tc.wantIDs)
			if !reflect.DeepEqual(gotIDs, tc.wantIDs) {
				t.Fatalf("shard %s: want %v, got %v", tc.shard, tc.wantIDs, gotIDs)
			}
		})
	}
}

// verify shards are disjoint and cover the full set
func TestApply_shardCoverage(t *testing.T) {
	m := 3
	seen := map[string]int{}
	for n := 1; n <= m; n++ {
		f, err := NewServiceFilter(nil, "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		f.shardN = n
		f.shardM = m
		for _, svc := range f.Apply(allServices) {
			seen[svc.Id]++
		}
	}
	for _, svc := range allServices {
		if seen[svc.Id] != 1 {
			t.Fatalf("service %s appeared %d times across shards (expected exactly 1)", svc.Id, seen[svc.Id])
		}
	}
}

// ── Apply: combined filters ───────────────────────────────────────────────────

func TestApply_combined(t *testing.T) {
	// allowlist: Production (keeps aaa, ccc, eee)
	// blocklist: C (removes ccc — "Production C")
	// shard 1/2: sorted [aaa, eee] → index 0 → aaa
	f, err := NewServiceFilter(nil, `^Production`, `C`, "1/2")
	if err != nil {
		t.Fatal(err)
	}
	got := f.Apply(allServices)
	if len(got) != 1 || got[0].Id != "aaa" {
		t.Fatalf("want [aaa], got %v", got)
	}
}
