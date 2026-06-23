package cmd

import (
	"fmt"
	"log/slog"
	"mogenius-operator/src/config"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/k8sexec"
	"mogenius-operator/src/shell"
	"os"
	"slices"
	"strconv"
	"strings"
)

// shellCandidates lists the shells tried, in order of preference, when the
// requested command is a bare shell-open request. The first one that exists in
// the container is used. This avoids failing on images that ship only bash or
// ash but no plain sh (and vice versa).
var shellCandidates = []string{"bash", "sh", "ash"}

type execArgs struct {
	Namespace string   `help:"" required:""`
	Pod       string   `help:"" required:""`
	Container string   `help:"" required:""`
	Command   []string `arg:"" help:"" default:"sh" passthrough:""`
}

func RunExec(args *execArgs, logger *slog.Logger, configModule config.ConfigModule) error {
	namespace := strings.TrimSpace(args.Namespace)
	if namespace == "" {
		return fmt.Errorf("empty --namespace")
	}

	container := strings.TrimSpace(args.Container)
	if container == "" {
		return fmt.Errorf("empty --container")
	}

	pod := strings.TrimSpace(args.Pod)
	if pod == "" {
		return fmt.Errorf("empty --pod")
	}

	clientProvider := k8sclient.NewK8sClientProvider(logger, configModule)
	executor, err := k8sexec.NewExecutor(
		logger,
		clientProvider.K8sClientSet().CoreV1().RESTClient(),
		*clientProvider.ClientConfig(),
		namespace,
		pod,
		container,
	)
	if err != nil {
		return err
	}

	// at least '--' and a single command string have to exist
	if len(args.Command) < 2 {
		return fmt.Errorf("missing command")
	}

	// require `--` to exist before passed commands
	if args.Command[0] != "--" {
		return fmt.Errorf("missing '--' before commands")
	}

	// trim the first leading `--`
	command := args.Command[1:]

	// A bare shell-open request (a single shell name) is resolved against the
	// shells actually present in the container, so we don't fail on images that
	// lack the exact shell requested. Explicit commands are run as-is.
	if len(command) == 1 && isShellName(command[0]) {
		shell, err := detectShell(executor, logger)
		if err != nil {
			logger.Error("no usable shell in container", "error", err)
			// Surface the original error in the terminal. This stdout write is
			// streamed to the browser via the pty before the process exits, i.e.
			// before the parent (cmdWait) emits the NO_SHELL_AVAILABLE signal.
			fmt.Printf("\r\n%s\r\n", strings.ReplaceAll(err.Error(), "\n", "\r\n"))
			// Exit with a dedicated code so the parent process can emit a
			// NO_SHELL_AVAILABLE signal to the frontend instead of a raw error.
			os.Exit(k8sexec.ShellNotFoundExitCode)
		}
		command = []string{shell}
	}

	fmt.Println(getConnectedBanner(namespace, pod, container, command))

	err = executor.Start(command)
	if err != nil {
		return err
	}

	return nil
}

// isShellName reports whether name is one of the known interactive shells,
// i.e. a bare shell-open request that should be resolved via detectShell.
func isShellName(name string) bool {
	return slices.Contains(shellCandidates, name)
}

// detectShell probes shellCandidates in order and returns the first shell that
// exists and is runnable inside the container. It returns an error (including
// the original per-shell runtime errors) if none are available (e.g. a
// distroless image with no shell at all).
func detectShell(executor k8sexec.Executor, logger *slog.Logger) (string, error) {
	var probeErrs []string
	for _, candidate := range shellCandidates {
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
		shellCandidates,
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

func getConnectedBanner(namespace string, pod string, container string, command []string) string {
	// +-+
	// | |
	// +-+
	// const tl = "+"
	// const tr = "+"
	// const bl = "+"
	// const br = "+"
	// const h = "-"
	// const v = "|"

	// ###
	// # #
	// ###
	// const tl = "#"
	// const tr = "#"
	// const bl = "#"
	// const br = "#"
	// const h = "#"
	// const v = "#"

	// ╭─╮
	// │ │
	// ╰─╯
	// const tl = "╭"
	// const tr = "╮"
	// const bl = "╰"
	// const br = "╯"
	// const h = "─"
	// const v = "│"

	// ┏━┓
	// ┃ ┃
	// ┗━┛
	// const tl = "┏"
	// const tr = "┓"
	// const bl = "┗"
	// const br = "┛"
	// const h = "━"
	// const v = "┃"

	// ╔═╗
	// ║ ║
	// ╚═╝
	const topLeft = "╔"
	const topRight = "╗"
	const bottomLeft = "╚"
	const bottomRight = "╝"
	const horizontal = "═"
	const vertical = "║"

	messages := []string{}
	messages = append(messages, "  "+"Namespace: "+namespace+"  ")
	messages = append(messages, "  "+"Pod:       "+pod+"  ")
	messages = append(messages, "  "+"Container: "+container+"  ")
	messages = append(messages, "  "+"Executing: "+fmt.Sprintf("%v", command)+"  ")

	messageLengths := []int{}
	for _, message := range messages {
		messageLengths = append(messageLengths, len(message))
	}

	longestMessage := slices.Max(messageLengths)

	paddedMessages := []string{}
	for _, message := range messages {
		paddedMessages = append(paddedMessages, fmt.Sprintf("%-"+strconv.Itoa(longestMessage)+"s", message))
	}

	hBorder := strings.Repeat(horizontal, longestMessage)
	hSpacer := strings.Repeat(" ", longestMessage)

	var banner string
	banner = banner + shell.BgBlack + shell.BoldCyan + topLeft + hBorder + topRight + shell.Reset + "\n"
	banner = banner + shell.BgBlack + shell.BoldCyan + vertical + hSpacer + vertical + shell.Reset + "\n"
	for _, message := range paddedMessages {
		banner = banner + shell.BgBlack + shell.BoldCyan + vertical + message + vertical + shell.Reset + "\n"
	}
	banner = banner + shell.BgBlack + shell.BoldCyan + vertical + hSpacer + vertical + shell.Reset + "\n"
	banner = banner + shell.BgBlack + shell.BoldCyan + bottomLeft + hBorder + bottomRight + shell.Reset + "\n"

	return banner
}
