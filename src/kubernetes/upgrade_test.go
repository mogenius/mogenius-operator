package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpgradeArgs(t *testing.T) {
	t.Parallel()

	base := []string{"upgrade", "--install", OPERATOR_RELEASE_NAME, OPERATOR_CHART_REF, "-n", "mogenius", "--reset-then-reuse-values"}
	dev := []string{"upgrade", "--install", OPERATOR_RELEASE_NAME, OPERATOR_CHART_REF + "-dev", "-n", "mogenius", "--reset-then-reuse-values", "--devel"}
	with := func(args []string, extra ...string) []string {
		return append(append([]string{}, args...), extra...)
	}

	tests := []struct {
		name    string
		current string
		target  string
		want    []string
		wantErr bool
	}{
		// older platforms send no version: latest release, as their command did
		{name: "no version: latest release", current: "2.34.0", target: "", want: base},
		{name: "no version on a develop build: latest dev build", current: "2.35.0-develop.12", target: "", want: dev},
		{name: "no version with an unknown running version", current: "NOT_SET", target: "", want: base},

		{name: "newer release", current: "2.34.0", target: "2.35.1", want: with(base, "--version", "2.35.1")},
		{name: "same release (reinstall)", current: "2.34.0", target: "2.34.0", want: with(base, "--version", "2.34.0")},
		{name: "v prefix is accepted", current: "2.34.0", target: "v2.35.0", want: with(base, "--version", "2.35.0")},
		{name: "local build metadata does not block", current: "2.34.0+dev", target: "2.34.0", want: with(base, "--version", "2.34.0")},
		{name: "newer develop build on a develop operator", current: "2.35.0-develop.12", target: "2.35.0-develop.13", want: with(dev, "--version", "2.35.0-develop.13")},
		{name: "release on a develop operator", current: "2.35.0-develop.12", target: "2.35.0", want: with(dev, "--version", "2.35.0")},

		{name: "downgrade", current: "2.34.0", target: "2.32.0", wantErr: true},
		{name: "older develop build", current: "2.35.0-develop.12", target: "2.35.0-develop.9", wantErr: true},
		{name: "pre-release for a release operator", current: "2.34.0", target: "2.35.0-develop.1", wantErr: true},
		{name: "not a version", current: "2.34.0", target: "latest", wantErr: true},
		{name: "flag injection", current: "2.34.0", target: "--post-renderer=/bin/sh", wantErr: true},
		{name: "partial version", current: "2.34.0", target: "2.35", wantErr: true},
		{name: "version against unknown running version", current: "NOT_SET", target: "2.35.0", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := UpgradeArgs(tt.current, tt.target, "mogenius")
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
