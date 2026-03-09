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

`IORIVER_API_TOKEN`: IO River API authentication token (required, unless the `-token` command-line option is included)

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
  -traffic-delay [30m0s]       Export IO River traffic metrics collected this time ago
  -traffic-timestamp [false]   Time series should be created with the traffic timestamp
  -verbose [false]             Print more information
  -version [false]             Print version information and exit
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
  -traffic-delay 30m \
  -verbose
```

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

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [Prometheus Go client](https://github.com/prometheus/client_golang)
- Uses [IORiver Go SDK](https://github.com/ioriver/ioriver-go)
