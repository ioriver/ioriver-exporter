package settings

import (
	"flag"
	"reflect"
	"sort"
	"testing"
)

func TestStringSliceFlag(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "single value",
			args: []string{"-service", "aaa"},
			want: []string{"aaa"},
		},
		{
			name: "repeatable",
			args: []string{"-service", "aaa", "-service", "bbb"},
			want: []string{"aaa", "bbb"},
		},
		{
			name: "comma-separated single invocation",
			args: []string{"-service", "aaa,bbb"},
			want: []string{"aaa", "bbb"},
		},
		{
			name: "mixed",
			args: []string{"-service", "aaa,bbb", "-service", "ccc"},
			want: []string{"aaa", "bbb", "ccc"},
		},
		{
			name: "whitespace trimmed",
			args: []string{"-service", " aaa , bbb "},
			want: []string{"aaa", "bbb"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ids []string
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.Var(stringSliceFlag{&ids}, "service", "")
			if err := fs.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			sort.Strings(ids)
			want := make([]string, len(tc.want))
			copy(want, tc.want)
			sort.Strings(want)
			if !reflect.DeepEqual(ids, want) {
				t.Fatalf("want %v, got %v", want, ids)
			}
		})
	}
}

// TestSupplementSettingsFromEnv verifies that env vars are applied with the
// correct precedence: CLI flag values take priority over env vars.
func TestSupplementSettingsFromEnv(t *testing.T) {
	setenv := func(t *testing.T, key, val string) {
		t.Helper()
		t.Setenv(key, val)
	}

	t.Run("env vars applied when flags not set", func(t *testing.T) {
		setenv(t, serviceIDsEnvVar, "id1, id2 , id3")
		setenv(t, serviceAllowlistEnvVar, "^Prod")
		setenv(t, serviceBlocklistEnvVar, "TEST")
		setenv(t, serviceShardEnvVar, "2/3")

		s := &Settings{}
		s.supplementSettingsFromEnv()

		wantIDs := []string{"id1", "id2", "id3"}
		sort.Strings(s.ServiceIDs)
		if !reflect.DeepEqual(s.ServiceIDs, wantIDs) {
			t.Errorf("ServiceIDs: want %v, got %v", wantIDs, s.ServiceIDs)
		}
		if s.ServiceAllowlist != "^Prod" {
			t.Errorf("ServiceAllowlist: want ^Prod, got %q", s.ServiceAllowlist)
		}
		if s.ServiceBlocklist != "TEST" {
			t.Errorf("ServiceBlocklist: want TEST, got %q", s.ServiceBlocklist)
		}
		if s.ServiceShard != "2/3" {
			t.Errorf("ServiceShard: want 2/3, got %q", s.ServiceShard)
		}
	})

	t.Run("CLI flag values take priority over env vars", func(t *testing.T) {
		setenv(t, serviceIDsEnvVar, "env-id")
		setenv(t, serviceAllowlistEnvVar, "env-allowlist")
		setenv(t, serviceBlocklistEnvVar, "env-blocklist")
		setenv(t, serviceShardEnvVar, "1/2")

		s := &Settings{
			ServiceIDs:       []string{"cli-id"},
			ServiceAllowlist: "cli-allowlist",
			ServiceBlocklist: "cli-blocklist",
			ServiceShard:     "2/2",
		}
		s.supplementSettingsFromEnv()

		if !reflect.DeepEqual(s.ServiceIDs, []string{"cli-id"}) {
			t.Errorf("ServiceIDs should not be overridden by env, got %v", s.ServiceIDs)
		}
		if s.ServiceAllowlist != "cli-allowlist" {
			t.Errorf("ServiceAllowlist should not be overridden by env, got %q", s.ServiceAllowlist)
		}
		if s.ServiceBlocklist != "cli-blocklist" {
			t.Errorf("ServiceBlocklist should not be overridden by env, got %q", s.ServiceBlocklist)
		}
		if s.ServiceShard != "2/2" {
			t.Errorf("ServiceShard should not be overridden by env, got %q", s.ServiceShard)
		}
	})

	t.Run("empty env vars leave fields unchanged", func(t *testing.T) {
		// Ensure vars are unset; t.Setenv restores original value on cleanup.
		t.Setenv(serviceIDsEnvVar, "")
		t.Setenv(serviceAllowlistEnvVar, "")
		t.Setenv(serviceBlocklistEnvVar, "")
		t.Setenv(serviceShardEnvVar, "")

		s := &Settings{}
		s.supplementSettingsFromEnv()

		if len(s.ServiceIDs) != 0 {
			t.Errorf("ServiceIDs should be empty, got %v", s.ServiceIDs)
		}
		if s.ServiceAllowlist != "" || s.ServiceBlocklist != "" || s.ServiceShard != "" {
			t.Errorf("filter fields should remain empty when env vars are unset")
		}
	})
}
