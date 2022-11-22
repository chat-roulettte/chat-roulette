package config

type TracingExporter string

const (
	TracingExporterJaeger    TracingExporter = "jaeger"
	TracingExporterHoneycomb TracingExporter = "honeycomb"
)
