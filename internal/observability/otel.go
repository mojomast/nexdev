package observability

import "fmt"

type OTelConfig struct {
	Enabled     bool
	Endpoint    string
	ServiceName string
}

type OTelShutdown func() error

// ConfigureOTel is intentionally inert unless explicitly enabled. The current
// v0.1 helper avoids SDK/exporter setup so tests never require network access.
func ConfigureOTel(cfg OTelConfig) (OTelShutdown, error) {
	if !cfg.Enabled {
		return func() error { return nil }, nil
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("observability.otel.endpoint is required when OTel is enabled")
	}
	return nil, fmt.Errorf("OpenTelemetry exporters are not wired in this build")
}
