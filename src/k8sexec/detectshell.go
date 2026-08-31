package k8sexec

import (
	"fmt"
	"log/slog"
	"strings"
)

// ShellCandidates lists the shells tried, in order of preference, when a
// caller needs one but the image's shell is unknown. The first one present in
// the container wins, so an image shipping only bash or only ash still works.
var ShellCandidates = []string{"bash", "sh", "ash"}

// DetectShell probes ShellCandidates in order and returns the first shell that
// exists and is runnable inside the container. It returns an error (including
// the original per-shell runtime errors) if none are available — a distroless
// image, typically.
func DetectShell(executor Executor, logger *slog.Logger) (string, error) {
	var probeErrs []string
	for _, candidate := range ShellCandidates {
		if err := executor.Probe([]string{candidate, "-c", "exit 0"}); err != nil {
			logger.Debug("shell not available in container", "shell", candidate, "error", err)
			probeErrs = append(probeErrs, cleanProbeError(err))
			continue
		}
		logger.Debug("using shell for interactive session", "shell", candidate)
		return candidate, nil
	}

	return "", fmt.Errorf(
		"no usable shell found in container (tried %v); the image may be distroless or ship without a shell\n%s",
		ShellCandidates,
		strings.Join(probeErrs, "\n"),
	)
}

// cleanProbeError strips the nested kubelet/CRI wrapping (and container ids)
// from a container exec error, leaving only the original runtime message,
// e.g. `exec: "sh": executable file not found in $PATH`.
func cleanProbeError(err error) string {
	msg := err.Error()
	if idx := strings.LastIndex(msg, "container process: "); idx >= 0 {
		msg = msg[idx+len("container process: "):]
	}
	// drop the appended exit-code suffix added by Probe, e.g.
	// " (command terminated with exit code 126)"
	if idx := strings.Index(msg, " (command terminated"); idx >= 0 {
		msg = msg[:idx]
	}
	return strings.TrimSpace(msg)
}
