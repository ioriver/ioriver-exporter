package settings

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

const (
	tokenEnvVar            = "IORIVER_API_TOKEN"
	listenEnvVar           = "IORIVER_LISTEN"
	serviceRefreshEnvVar   = "IORIVER_SERVICE_REFRESH"
	trafficDelayEnvVar     = "IORIVER_TRAFFIC_DELAY"
	trafficTimestampEnvVar = "IORIVER_TRAFFIC_TIMESTAMP"
	verboseEnvVar          = "IORIVER_VERBOSE"
)

const (
	defaultListen         = "127.0.0.1:8080"
	defaultServiceRefresh = 1 * time.Minute
	defaultTrafficDelay   = 30 * time.Minute
)

type Settings struct {
	Token            string
	Listen           string
	ServiceRefresh   time.Duration
	TrafficDelay     time.Duration
	TrafficTimestamp bool
	Verbose          bool
	Version          bool
}

// CollectSettings collects settings from the CLI and environment
func CollectSettings(name string) (*Settings, error) {
	settings := Settings{}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)

	tokenUsage := fmt.Sprintf("IORiver API token (required unless set by %s)", tokenEnvVar)

	fs.StringVar(&settings.Token, "token", "", tokenUsage)
	fs.StringVar(&settings.Listen, "listen", defaultListen, "listen address for HTTP requests")
	fs.DurationVar(&settings.ServiceRefresh, "service-refresh", defaultServiceRefresh, "how often to poll IORiver to refresh the list of services (15s–10m)")
	fs.DurationVar(&settings.TrafficDelay, "traffic-delay", defaultTrafficDelay, "export IORiver traffic metrics collected this time ago")

	fs.BoolVar(&settings.TrafficTimestamp, "traffic-timestamp", false, "time series should be created with the traffic timestamp")
	fs.BoolVar(&settings.Verbose, "verbose", false, "print more information")
	fs.BoolVar(&settings.Version, "version", false, "print version information and exit")

	fs.Usage = getUsageFunc(fs, name)

	err := fs.Parse(os.Args[1:])

	if err == nil {
		settings.supplementSettingsFromEnv()
	}

	return &settings, err
}

// Validate validates all settings
func (s *Settings) Validate(logger log.Logger) bool {
	isValid := true
	if s.Token == "" {
		level.Error(logger).Log("err", fmt.Sprintf("-token or %s is required", tokenEnvVar))
		isValid = false
	}
	if s.ServiceRefresh < 15*time.Second {
		level.Warn(logger).Log("warn", "-service-refresh cannot be shorter than 15s; set default value")
		s.ServiceRefresh = defaultServiceRefresh
	}
	return isValid
}

// supplement the settings from the environment
func (s *Settings) supplementSettingsFromEnv() {
	if s.Token == "" {
		s.Token = os.Getenv(tokenEnvVar)
	}
	if s.Listen == defaultListen {
		if addr := os.Getenv(listenEnvVar); addr != "" {
			s.Listen = addr
		}
	}
	if s.ServiceRefresh == 1*time.Minute {
		if refresh := os.Getenv(serviceRefreshEnvVar); refresh != "" {
			if d, err := time.ParseDuration(refresh); err == nil {
				s.ServiceRefresh = d
			}
		}
	}
	if s.TrafficDelay == defaultTrafficDelay {
		if delay := os.Getenv(trafficDelayEnvVar); delay != "" {
			if d, err := time.ParseDuration(delay); err == nil {
				s.TrafficDelay = d
			}
		}
	}
	if !s.TrafficTimestamp {
		if ts := os.Getenv(trafficTimestampEnvVar); ts == "true" || ts == "1" {
			s.TrafficTimestamp = true
		}
	}
	if !s.Verbose {
		if v := os.Getenv(verboseEnvVar); v == "true" || v == "1" {
			s.Verbose = true
		}
	}
}

// get function to print the USAGE section
func getUsageFunc(fs *flag.FlagSet, name string) func() {
	return func() {
		out := os.Stdout
		fmt.Fprintf(out, "USAGE\n")
		fmt.Fprintf(out, "  %s [OPTIONS]\n", name)
		fmt.Fprintf(out, "\n")
		fmt.Fprintf(out, "OPTIONS\n")

		tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
		fs.VisitAll(func(f *flag.Flag) {
			fmt.Fprintf(tw, "  -%s [%s]\t%s\n", f.Name, f.DefValue, f.Usage)
		})
		tw.Flush()

		fmt.Fprintf(out, "\n")
	}
}
