# ioriver-exporter

[![Go Version](https://img.shields.io/badge/Go-1.24-blue.svg)](https://golang.org/)

A Prometheus exporter exposing metrics and traffic statistics of [IORiver](https://ioriver.io/) services.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
  - [Docker](#docker)
  - [Kubernetes](#kubernetes)
  - [From Source](#from-source)
- [Authentication](#authentication)
- [Usage and Command-Line Options](#usage-and-command-line-options)
- [Examples](#examples)
- [Metrics](#metrics)
- [Grafana Dashboards](#grafana-dashboards)
- [Integrations](#integrations)
  - [OpenTelemetry Collector](#opentelemetry-collector)
  - [Datadog](#datadog)
    - [Datadog Dashboard](#datadog-dashboard)
- [License](#license)

## Features

- **Real-time Metrics**: Continuously polls IO River API for traffic statistics
- **Multi-Service Support**: Automatically discovers and monitors all services in your IO River account
- **Prometheus Integration**: Native Prometheus metrics format
- **Optional Timestamps**: Support for historical metric timestamps

## Installation

### Docker

Available on the [packages page](https://github.com/ioriver/ioriver-exporter/pkgs/container/ioriver-exporter).

```sh
docker pull ghcr.io/ioriver/ioriver-exporter:latest
```

### From Source

```bash
git clone https://github.com/ioriver-dev/ioriver-exporter.git
cd ioriver-exporter
go build -o ioriver-exporter ./cmd/ioriver-exporter
```

### Kubernetes

The example below deploys ioriver-exporter as a `Deployment` with a `ClusterIP` Service. The Service is required when scraping via the [OpenTelemetry Collector](#opentelemetry-collector) or [Datadog](#datadog) — it provides a stable DNS endpoint (`ioriver-exporter.<namespace>.svc.cluster.local`).

The API token is read from a Kubernetes Secret. Create it before deploying:

```bash
kubectl create secret generic ioriver-exporter \
  --from-literal=IORIVER_API_TOKEN=<your-api-token> \
  -n <namespace>
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ioriver-exporter
  namespace: <namespace>
  labels:
    app: ioriver-exporter
spec:
  selector:
    matchLabels:
      app: ioriver-exporter
  template:
    metadata:
      labels:
        app: ioriver-exporter
    spec:
      containers:
        - name: ioriver-exporter
          image: ghcr.io/ioriver/ioriver-exporter:latest
          imagePullPolicy: Always
          ports:
            - name: metrics
              containerPort: 8080
              protocol: TCP
          env:
            - name: IORIVER_LISTEN
              value: "0.0.0.0:8080"
            - name: IORIVER_TRAFFIC_TIMESTAMP
              value: "true"
            # Optional: restrict to specific services by name regex
            # - name: IORIVER_SERVICE_ALLOWLIST
            #   value: "^production.*"
          envFrom:
            - secretRef:
                name: ioriver-exporter   # must contain IORIVER_API_TOKEN
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 128Mi
          livenessProbe:
            httpGet:
              path: /
              port: 8080
          readinessProbe:
            httpGet:
              path: /
              port: 8080
          startupProbe:
            httpGet:
              path: /
              port: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: ioriver-exporter
  namespace: <namespace>
  labels:
    app: ioriver-exporter
spec:
  selector:
    app: ioriver-exporter
  ports:
    - name: metrics
      port: 80
      targetPort: 8080
      protocol: TCP
```

Once deployed, the metrics endpoint is reachable at:
- **Within the cluster:** `http://ioriver-exporter.<namespace>.svc.cluster.local/metrics`
- **Port-forward for local testing:** `kubectl port-forward svc/ioriver-exporter 8080:80 -n <namespace>`

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
docker run --detach --publish 8080:8080 --env IORIVER_API_TOKEN=<your-api-token> ghcr.io/ioriver/ioriver-exporter:latest
```

Run with custom options

```bash
# Custom metrics address and refresh intervals
./ioriver-exporter \
  -token "<your-api-token>" \
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

- `serviceID`: IO River service ID
- `serviceName`: IO River service name
- `providerName`: CDN provider name

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

```promql
last_over_time(ioriver_traffic_hits{serviceID="0fb49f03-5078-4f44-ad3f-623a82184d93", providerName="Fastly"}[1m])
```

## Grafana Dashboards

Pre-built Grafana dashboards are available in the [`dashboards/`](dashboards/) directory.

### IORiver Exporter Dashboard

The main dashboard (`dashboards/ioriver-exporter.json`) visualises traffic across all CDN providers for your IORiver services.

**Importing the dashboard:**
1. In Grafana, go to **Dashboards → Import**.
2. Upload `dashboards/ioriver-exporter.json` or paste its contents.
3. Select the `prometheus` datasource when prompted.
4. Click **Import**.

**Template variables:**

| Variable | Description |
|---|---|
| `serviceName` | Filter by IORiver service name |
| `providerName` | Filter by CDN provider name |

**Panels:**
- **Traffic Overview** — Request hits and bytes served over time, per provider
- **Cache Performance** — Cached hits and bytes percentages
- **Error Rate** — Error percentage over time
- **Status Code Breakdown** — Hits and bytes by HTTP status code
- **Protocol & Method** — Hits and bytes by HTTP version and method
- **Origin Traffic** — Origin hits and bytes (cache miss traffic)

## Integrations

### OpenTelemetry Collector

You can forward IORiver metrics to any OTel-compatible backend (Grafana Cloud, Honeycomb, New Relic, Datadog OTLP endpoint, ClickHouse via HyperDX/ClickStack, etc.) by running the exporter alongside an [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/).

**Important:** enable `IORIVER_TRAFFIC_TIMESTAMP=true` (or `-traffic-timestamp`) so the exporter embeds the actual CDN data timestamp in each metric. Without this, all metrics will be stamped with the scrape time, which masks the provider-specific delays described in [Data Delays](#data-delays).

Add the following to your OTel Collector configuration:

```yaml
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: ioriver-exporter
          scrape_interval: 60s        # matches the IORiver API refresh cadence
          scrape_timeout: 30s
          honor_timestamps: true      # preserves CDN data timestamps set by -traffic-timestamp
          static_configs:
            # Use the pod IP:8080 for direct scraping, or the Service DNS:80 if using a ClusterIP Service
            - targets: ['<ioriver-exporter-host>:8080']

exporters:
  otlphttp:
    endpoint: https://<your-otel-backend>

service:
  pipelines:
    metrics:
      receivers: [prometheus]
      exporters: [otlphttp]
```

All `ioriver_traffic_*` metrics are of type `Gauge` and will be stored in your backend's gauge metric table.

### Datadog

#### Kubernetes (recommended)

On Kubernetes, use [Datadog Autodiscovery (AD v2)](https://docs.datadoghq.com/containers/kubernetes/prometheus/?tab=kubernetesadv2) to configure the OpenMetrics check via pod annotations — no separate config file needed. The Datadog Agent detects the annotation and starts scraping automatically.

Add the following annotation to the ioriver-exporter pod template:

```yaml
annotations:
  ad.datadoghq.com/ioriver-exporter.checks: |
    {
      "openmetrics": {
        "instances": [
          {
            "openmetrics_endpoint": "http://%%host%%:%%port%%/metrics",
            "namespace": "ioriver",
            "metrics": ["ioriver_traffic_.*"],
            "honor_timestamps": true
          }
        ]
      }
    }
```

`%%host%%` and `%%port%%` are Datadog template variables resolved automatically at scrape time. The container name in the annotation key (`ioriver-exporter`) must match the actual container name in the pod spec.

> **Note:** Requires Datadog Agent v7.36+ for AD v2 annotations. For older agents, use the [AD v1 format](https://docs.datadoghq.com/containers/kubernetes/prometheus/?tab=kubernetesv1).

#### Non-Kubernetes

For standalone Datadog Agent deployments, create a conf file at `/etc/datadog-agent/conf.d/ioriver_exporter.d/conf.yaml`:

```yaml
init_config:

instances:
  - openmetrics_endpoint: http://<ioriver-exporter-host>:8080/metrics
    namespace: ioriver
    metrics:
      - ioriver_traffic_.*
    honor_timestamps: true
```

Restart the agent and verify: `datadog-agent check ioriver_exporter`

#### Notes

- All metrics appear in Datadog under the prefix `ioriver.*` (e.g. `ioriver.ioriver_traffic_bytes`)
- Enable `-traffic-timestamp` on the exporter — `honor_timestamps: true` in the check config preserves CDN data timestamps, so metrics are correctly placed on the time axis despite provider-specific delays

#### Datadog Dashboard

A pre-built Datadog dashboard is available at [`dashboards/datadog/ioriver-exporter.json`](dashboards/datadog/ioriver-exporter.json). It covers:

- **Traffic Overview** — hits and bytes per service / provider
- **Cache Performance** — cache hit rate and cached bytes percentage
- **Error Rate** — error percentage with warning (1%) and critical (5%) markers
- **Status Code Breakdown** — hits and bytes by HTTP status code
- **Protocol & Method** — hits by HTTP version and HTTP method
- **Origin Traffic** — origin hits and bytes per service / provider

**Importing the dashboard:**
1. In Datadog, go to **Dashboards → New Dashboard → Import dashboard JSON**.
2. Paste the contents of `dashboards/datadog/ioriver-exporter.json`, or drag-and-drop the file.
3. Click **Yes, Replace** to confirm.

The `serviceName` and `providerName` template variables are pre-configured and will auto-populate with the values from your own metrics once data is flowing.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [Prometheus Go client](https://github.com/prometheus/client_golang)
- Uses [IORiver Go SDK](https://github.com/ioriver/ioriver-go)
