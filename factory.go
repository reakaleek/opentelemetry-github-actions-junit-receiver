package opentelemetrygithubactionsjunitreceiver

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

func createDefaultConfig() component.Config {
	return &Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: "localhost:33333",
		},
		Path: "/githubactionsjunit",
	}
}

func createTracerReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	rConf component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	cfg := rConf.(*Config)
	return newTracesReceiver(cfg, params, nextConsumer)
}

// NewFactory creates a factory for githubactionslogsreceiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		component.MustNewType("githubactionsjunit"),
		createDefaultConfig,
		receiver.WithTraces(createTracerReceiver, component.StabilityLevelAlpha),
	)
}
