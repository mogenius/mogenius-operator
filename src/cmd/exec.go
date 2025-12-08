package cmd

import (
	"fmt"
	"log/slog"
	"mogenius-operator/src/config"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/k8sexec"
	"mogenius-operator/src/shell"
	"slices"
	"strconv"
	"strings"
)

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

	// at least '--' and a single command string have to exist
	if len(args.Command) < 2 {
		return fmt.Errorf("missing command")
	}

	// require `--` to exist before passed commands
	if args.Command[0] != "--" {
		return fmt.Errorf("missing '--' before commands")
	}

	// trim the first leading `--`
	args.Command = args.Command[1:]

	fmt.Println(getConnectedBanner(namespace, pod, container, args.Command))

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

	err = executor.Start(args.Command)
	if err != nil {
		return err
	}

	return nil
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
