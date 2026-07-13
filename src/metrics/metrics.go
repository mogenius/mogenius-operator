package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var patternDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "mogenius_operator_pattern_duration_seconds",
		Help:    "Duration of pattern handler executions.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"path"},
)

var websocketConnected = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "mogenius_operator_websocket_connected",
		Help: "Whether the websocket connection is established (1=connected, 0=disconnected).",
	},
	[]string{"connection"},
)

var auditLogEntriesWritten = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "mogenius_operator_audit_log_entries_written_total",
		Help: "Audit log entries successfully persisted, by source (api, ai-chat).",
	},
	[]string{"source"},
)

var auditLogWriteFailures = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "mogenius_operator_audit_log_write_failures_total",
		Help: "Audit log entries that could not be persisted. The audit log is best effort; a rising counter means mutations happen without an audit trail.",
	},
)

var auditLogEventsDropped = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "mogenius_operator_audit_log_events_dropped_total",
		Help: "Real-time audit log events dropped because the dispatcher queue was full (the persisted entry is unaffected).",
	},
)

func IncAuditLogWritten(source string) {
	auditLogEntriesWritten.WithLabelValues(source).Inc()
}

func IncAuditLogWriteFailure() {
	auditLogWriteFailures.Inc()
}

func IncAuditLogEventDropped() {
	auditLogEventsDropped.Inc()
}

func ObservePatternDuration(pattern string, seconds float64) {
	patternDuration.WithLabelValues(pattern).Observe(seconds)
}

func SetWebsocketConnected(name string, connected bool) {
	if name == "" {
		return
	}
	val := 0.0
	if connected {
		val = 1.0
	}
	websocketConnected.WithLabelValues(name).Set(val)
}

var reconcileDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "mogenius_operator_reconcile_duration_seconds",
		Help:    "Duration of a single reconcile call, by resource kind and operation.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"resource", "operation"},
)

var reconcileTrackedObjects = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "mogenius_operator_reconcile_tracked_objects",
		Help: "Number of objects currently tracked in the reconciler cache, by resource kind.",
	},
	[]string{"resource"},
)

var reconcileQueueWait = promauto.NewHistogram(
	prometheus.HistogramOpts{
		Name:    "mogenius_operator_reconcile_queue_wait_seconds",
		Help:    "Time spent waiting to acquire a reconcile concurrency slot. Spikes indicate the reconciler is saturated.",
		Buckets: prometheus.DefBuckets,
	},
)

func ObserveReconcileDuration(resource, operation string, seconds float64) {
	reconcileDuration.WithLabelValues(resource, operation).Observe(seconds)
}

func SetReconcileTrackedObjects(resource string, count int) {
	reconcileTrackedObjects.WithLabelValues(resource).Set(float64(count))
}

func ObserveReconcileQueueWait(seconds float64) {
	reconcileQueueWait.Observe(seconds)
}
