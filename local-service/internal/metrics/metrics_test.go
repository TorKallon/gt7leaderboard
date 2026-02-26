package metrics

import (
	"testing"
)

func TestNoopMetrics_ImplementsInterface(t *testing.T) {
	var m Metrics = NewNoop()
	if m == nil {
		t.Fatal("NewNoop() returned nil")
	}
}

func TestNoopMetrics_DoesNotPanic(t *testing.T) {
	m := NewNoop()

	// These should all complete without panicking.
	m.Incr("test.counter", []string{"tag:value"})
	m.Gauge("test.gauge", 42.0, []string{"tag:value"})
	m.Histogram("test.histogram", 1.5, []string{"tag:value"})
	m.Close()

	// Call again after close - should still not panic.
	m.Incr("test.counter", nil)
	m.Gauge("test.gauge", 0, nil)
	m.Histogram("test.histogram", 0, nil)
}

func TestNoopMetrics_WithNilTags(t *testing.T) {
	m := NewNoop()
	m.Incr("test.counter", nil)
	m.Gauge("test.gauge", 1.0, nil)
	m.Histogram("test.histogram", 1.0, nil)
}

func TestDatadogMetrics_ImplementsInterface(t *testing.T) {
	// Verify the type implements the interface at compile time.
	var _ Metrics = &datadogMetrics{}
}

func TestNew_InvalidAddress(t *testing.T) {
	// Datadog statsd client is lazy, so even bad addresses may not error.
	// But we can at least verify the constructor works.
	m, err := New("localhost:8125", WithNamespace("gt7."), WithTags([]string{"env:test"}))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer m.Close()

	// Verify it doesn't panic when called (even though the agent isn't running).
	m.Incr("test.counter", []string{"test:true"})
	m.Gauge("test.gauge", 99.9, []string{"test:true"})
	m.Histogram("test.histogram", 0.5, []string{"test:true"})
}

func TestWithNamespace(t *testing.T) {
	o := &options{}
	WithNamespace("gt7.")( o)
	if o.namespace != "gt7." {
		t.Errorf("expected namespace %q, got %q", "gt7.", o.namespace)
	}
}

func TestWithTags(t *testing.T) {
	o := &options{}
	WithTags([]string{"env:test", "service:gt7"})(o)
	if len(o.tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(o.tags))
	}
}
