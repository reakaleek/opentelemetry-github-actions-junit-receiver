# opentelemetry-github-actions-junit-receiver


## Local Development

### Prerequisites

- [Go](https://golang.org/dl/)
- [OpenTelemetry Collector Builder (OCB)](https://opentelemetry.io/docs/collector/custom-collector/#step-1---install-the-builder)

### Build the collector

```shell
make build
```

### Run the collector

```shell
make run WEBHOOK_SECRET=your-webhook-secret GITHUB_TOKEN=your-github-token
```

### Configure your GitHub repository for testing purposes

TBC