package services

import (
	"context"
	"fmt"
	"io"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/xterm"
	"strings"

	"k8s.io/client-go/rest"
)

func GetPreviousLogContent(podCmdConnectionRequest xterm.PodCmdConnectionRequest) io.Reader {
	ctx := context.Background()
	cancelCtx, endGofunc := context.WithCancel(ctx)
	defer endGofunc()

	pod := kubernetes.PodStatus(podCmdConnectionRequest.Namespace, podCmdConnectionRequest.Pod, false)
	terminatedState := kubernetes.LastTerminatedStateIfAny(pod)

	var previousRestReq *rest.Request
	if terminatedState != nil {
		tmpPreviousResReq, err := PreviousPodLogStream(podCmdConnectionRequest.Namespace, podCmdConnectionRequest.Pod)
		if err != nil {
			serviceLogger.Error(err.Error())
		} else {
			previousRestReq = tmpPreviousResReq
		}
	}

	if previousRestReq == nil {
		return nil
	}

	var previousStream io.ReadCloser
	tmpPreviousStream, err := previousRestReq.Stream(cancelCtx)
	if err != nil {
		serviceLogger.Error(err.Error())
		previousStream = io.NopCloser(strings.NewReader(fmt.Sprintln(err.Error())))
	} else {
		previousStream = tmpPreviousStream
	}

	data, err := io.ReadAll(previousStream)
	if err != nil {
		serviceLogger.Error("failed to read data", "error", err)
	}

	lastState := kubernetes.LastTerminatedStateToString(terminatedState)

	nl := strings.NewReader("\r\n")
	previousState := strings.NewReader(lastState)
	headlineLastLog := strings.NewReader("Last Log:\r\n")
	headlineCurrentLog := strings.NewReader("\r\nCurrent Log:\r\n")

	return io.MultiReader(previousState, nl, headlineLastLog, strings.NewReader(string(data)), nl, headlineCurrentLog)
}

func XTermClusterToolStreamConnection(buildLogConnectionRequest xterm.ClusterToolConnectionRequest) {
	xterm.XTermClusterToolStreamConnection(
		buildLogConnectionRequest.WsConnection,
		buildLogConnectionRequest.CmdType,
		buildLogConnectionRequest.Tool,
	)
}
