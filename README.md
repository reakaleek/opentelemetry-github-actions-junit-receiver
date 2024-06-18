# opentelemetry-github-actions-junit-receiver


## Local Development

### Prerequisites

- [Go](https://golang.org/dl/)
- [OpenTelemetry Collector Builder (OCB)](https://opentelemetry.io/docs/collector/custom-collector/#step-1---install-the-builder)

### Build the collector

```shell
ocb --config builder-config.yml
```

### Run the collector

```shell
./bin/otelcol-custom --config config.yml
```
