package metrics

import (
	"github.com/DataDog/datadog-go/v5/statsd"
)

// Metrics provides an interface for recording application metrics.
type Metrics interface {
	Incr(name string, tags []string)
	Gauge(name string, value float64, tags []string)
	Histogram(name string, value float64, tags []string)
	Close()
}

// Option configures a Metrics instance.
type Option func(*options)

type options struct {
	namespace string
	tags      []string
}

// WithNamespace sets the metric namespace prefix.
func WithNamespace(ns string) Option {
	return func(o *options) {
		o.namespace = ns
	}
}

// WithTags sets default tags applied to all metrics.
func WithTags(tags []string) Option {
	return func(o *options) {
		o.tags = tags
	}
}

// datadogMetrics wraps a Datadog statsd client.
type datadogMetrics struct {
	client *statsd.Client
}

// New creates a new Datadog-backed Metrics implementation.
// The addr parameter is the statsd agent address (e.g., "localhost:8125").
func New(addr string, opts ...Option) (Metrics, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	var statsdOpts []statsd.Option
	if o.namespace != "" {
		statsdOpts = append(statsdOpts, statsd.WithNamespace(o.namespace))
	}
	if len(o.tags) > 0 {
		statsdOpts = append(statsdOpts, statsd.WithTags(o.tags))
	}

	client, err := statsd.New(addr, statsdOpts...)
	if err != nil {
		return nil, err
	}

	return &datadogMetrics{client: client}, nil
}

func (d *datadogMetrics) Incr(name string, tags []string) {
	d.client.Incr(name, tags, 1)
}

func (d *datadogMetrics) Gauge(name string, value float64, tags []string) {
	d.client.Gauge(name, value, tags, 1)
}

func (d *datadogMetrics) Histogram(name string, value float64, tags []string) {
	d.client.Histogram(name, value, tags, 1)
}

func (d *datadogMetrics) Close() {
	d.client.Close()
}

// noopMetrics is a no-op implementation of Metrics that discards all data.
type noopMetrics struct{}

// NewNoop creates a no-op Metrics implementation that silently discards all metrics.
func NewNoop() Metrics {
	return &noopMetrics{}
}

func (n *noopMetrics) Incr(name string, tags []string)                  {}
func (n *noopMetrics) Gauge(name string, value float64, tags []string)  {}
func (n *noopMetrics) Histogram(name string, value float64, tags []string) {}
func (n *noopMetrics) Close()                                           {}
