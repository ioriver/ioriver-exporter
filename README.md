# ioriver-exporter

[![Go Version](https://img.shields.io/badge/Go-1.24-blue.svg)](https://golang.org/)

A Prometheus exporter exposing metrics and traffic statistics of [IORiver](https://ioriver.io/) services.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Authentication](#authentication)
- [Usage and Command-Line Options](#usage-and-command-line-options)
- [Example of Usage](#example-of-usage)
- [Metrics](#metrics)
- [License](#license)

## Features

- **Real-time Metrics**: Continuously polls IO River API for traffic statistics
- **Multi-Service Support**: Automatically discovers and monitors all services in your IO River account
- **Prometheus Integration**: Native Prometheus metrics format
- **Optional Timestamps**: Support for historical metric timestamps

## Installation

### Docker

Available on the [packages page](https://github.com/ioriver-dev/ioriver-exporter/pkgs/container/ioriver-exporter).

```sh
docker pull ghcr.io/ioriver-dev/ioriver-exporter
```

### From Source

```bash
git clone https://github.com/ioriver-dev/ioriver-exporter.git
cd ioriver-exporter
go build -o ioriver-exporter ./cmd/ioriver-exporter
```

## Authentication

### Environment Variable

```
  IORIVER_API_TOKEN:           IO River API authentication token (required, unless the `-token` command-line option is included)
  IORIVER_LISTEN:              Listen address for HTTP requests
  IORIVER_SERVICE_REFRESH:     How often to poll IO River to refresh the list of services (15s–10m)
  IORIVER_TRAFFIC_TIMESTAMP:   Time series should be created with the traffic timestamp
  IORIVER_VERBOSE:             Print more information
  IORIVER_SERVICE_IDS:         Comma-separated list of service IDs to export (default: all)
  IORIVER_SERVICE_ALLOWLIST:   Export only services whose name matches this regex
  IORIVER_SERVICE_BLOCKLIST:   Exclude services whose name matches this regex
  IORIVER_SERVICE_SHARD:       Shard services across exporter instances, e.g. 1/3
```

### Command-Line Option

```
OPTIONS
  -token [string]              IO River API token (required unless set by IORIVER_API_TOKEN)
```

## Usage and Command-Line Options

```
OPTIONS
  -token [string]              IO River API token (required unless set by IORIVER_API_TOKEN)
  -listen [127.0.0.1:8080]     Listen address for HTTP requests
  -service-refresh [1m0s]      How often to poll IO River to refresh the list of services (15s–10m)
  -traffic-timestamp [false]   Time series should be created with the traffic timestamp
  -verbose [false]             Print more information
  -version [false]             Print version information and exit
  -service [string]            Export only this service ID (repeatable and/or comma-separated; default: all)
  -service-allowlist [string]  Export only services whose name matches this regex
  -service-blocklist [string]  Exclude services whose name matches this regex
  -service-shard [string]      Shard services across exporter instances, e.g. 1/3
```

## Examples

Run in Docker (recommended)

```bash
docker run --detach --publish 8080:8080 --env IORIVER_API_TOKEN=emxpdcbe7a83b537ac696442d9f82a9137542d1049d0c781 ghcr.io/ioriver-dev/ioriver-exporter
```

Run with custom options

```bash
# Custom metrics address and refresh intervals
./ioriver-exporter \
  -token "your-api-token" \
  -listen "127.0.0.1:8080" \
  -service-refresh 30s \
  -verbose
```

## Service Filtering

By default all services accessible to the API token are exported. You can restrict this with four flags that are applied in order:

1. **`-service <id>`** — export only the listed service ID(s). Repeatable and comma-separated:
   ```bash
   ./ioriver-exporter -token ... -service aaa-111 -service bbb-222
   ./ioriver-exporter -token ... -service aaa-111,bbb-222
   ```

2. **`-service-allowlist '<regex>'`** — keep only services whose **name** matches the regex:
   ```bash
   ./ioriver-exporter -token ... -service-allowlist '^Production'
   ```

3. **`-service-blocklist '<regex>'`** — exclude services whose **name** matches the regex:
   ```bash
   ./ioriver-exporter -token ... -service-blocklist '.*TEST.*'
   ```

4. **`-service-shard n/m`** — distribute services deterministically across `m` exporter instances. Run one instance per shard:
   ```bash
   ./ioriver-exporter [common flags] -service-shard 1/3
   ./ioriver-exporter [common flags] -service-shard 2/3
   ./ioriver-exporter [common flags] -service-shard 3/3
   ```
   Services are sorted alphabetically by ID before sharding, so the assignment is stable across restarts.

All four flags can be combined; they are evaluated in the order listed above.

## Metrics

All metrics are prefixed with `ioriver_traffic_` and include the following labels:

- `service_id`: IO River service ID
- `provider`: CDN provider name

### Available Metrics

| Metric                                    | Type  | Description                        |
|-------------------------------------------|-------|------------------------------------|
| `ioriver_traffic_bytes`                   | Gauge | Total bytes served                 |
| `ioriver_traffic_cached_bytes_percentage` | Gauge | Cached bytes percentage            |
| `ioriver_traffic_bytes_by_http_method`    | Gauge | Total bytes served by HTTP method  |
| `ioriver_traffic_bytes_by_http_version`   | Gauge | Total bytes served by HTTP version |
| `ioriver_traffic_bytes_by_status_code`    | Gauge | Total bytes served by status code  |
| `ioriver_traffic_hits`                    | Gauge | Total hits served                  |
| `ioriver_traffic_cached_hits_percentage`  | Gauge | Cached hits percentage             |
| `ioriver_traffic_hits_by_http_method`     | Gauge | Total hits served by HTTP method   |
| `ioriver_traffic_hits_by_http_version`    | Gauge | Total hits served by HTTP version  |
| `ioriver_traffic_hits_by_status_code`     | Gauge | Total hits served by status code   |
| `ioriver_traffic_errors_percentage`       | Gauge | Error percentage                   |

## Data Delays

Traffic metrics are collected periodically from each configured CDN provider: 
- Due to provider-specific processing pipelines, metric availability is subject to inherent delays. 
- These delays vary by provider, ranging from a few minutes up to approximately one hour before data becomes accessible via their APIs.

The IO River Prometheus provider accounts for these differences by:
- Dynamically polling each CDN provider.
- Ingesting metrics as soon as they become available, rather than assuming uniform availability across sources.

It is important to note that Prometheus, by default, continues to display the most recent data point when no new data has been received. This behavior can lead to misinterpretation, as it may appear that metrics are current when they are not.

To ensure that queries reflect only explicitly retrieved data points, it is recommended to use the [last_over_time](https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time) function. For example:

```last_over_time(ioriver_traffic_hits{serviceID="0fb49f03-5078-4f44-ad3f-623a82184d93", providerName="Fastly"}[1m])```

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [Prometheus Go client](https://github.com/prometheus/client_golang)
- Uses [IORiver Go SDK](https://github.com/ioriver/ioriver-go)
