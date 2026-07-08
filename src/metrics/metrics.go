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
