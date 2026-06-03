package core

import "testing"

func TestResolvePrometheusRange(t *testing.T) {
	tests := []struct {
		name              string
		timeOffsetSeconds int64
		step              int
		wantOffset        int64
		wantStep          int
	}{
		{
			name:              "24h with stepped step stays within point limit",
			timeOffsetSeconds: 86400,
			step:              1440,
			wantOffset:        86400,
			wantStep:          1440, // 60 points
		},
		{
			name:              "zero step falls back to ~60 points",
			timeOffsetSeconds: 3600,
			step:              0,
			wantOffset:        3600,
			wantStep:          60, // 3600/60
		},
		{
			name:              "negative step falls back to ~60 points",
			timeOffsetSeconds: 3600,
			step:              -5,
			wantOffset:        3600,
			wantStep:          60,
		},
		{
			name:              "step larger than half the range is recalculated",
			timeOffsetSeconds: 3600,
			step:              3000,
			wantOffset:        3600,
			wantStep:          60,
		},
		{
			name:              "tiny offset is clamped to 60s minimum",
			timeOffsetSeconds: 10,
			step:              0,
			wantOffset:        60,
			wantStep:          1,
		},
		{
			name:              "large range with small step is clamped to point limit",
			timeOffsetSeconds: 2592000, // 30 days
			step:              60,      // would be 43200 points -> rejected by Prometheus
			wantOffset:        2592000,
			wantStep:          236, // ceil(2592000/11000) -> 10983 points, within the limit
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOffset, gotStep := resolvePrometheusRange(tt.timeOffsetSeconds, tt.step)
			if gotOffset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", gotOffset, tt.wantOffset)
			}
			if gotStep != tt.wantStep {
				t.Errorf("step = %d, want %d", gotStep, tt.wantStep)
			}
			// Invariant that matters most: never exceed Prometheus' point limit.
			if points := tt.timeOffsetSeconds / int64(gotStep); points > prometheusMaxPoints {
				t.Errorf("resolved to %d points, exceeds limit %d (step=%d)", points, prometheusMaxPoints, gotStep)
			}
		})
	}
}
