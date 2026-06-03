package core

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
