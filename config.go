package opentelemetrygithubactionsjunitreceiver

import (
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
)

type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`
	Path                    string
	WebhookSecret           configopaque.String `mapstructure:"webhook_secret"`
	Token                   configopaque.String `mapstructure:"token"`
}
