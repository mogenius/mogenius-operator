package sshgateway

import (
	"strconv"

	"k8s.io/client-go/tools/remotecommand"
)

// execParams describes one exec call into the target container. The stream
// flags differ per caller — an interactive session wants a TTY and merges
// stderr into it, a file operation wants stderr separate — so they are part
// of the request rather than assumed.
type execParams struct {
	namespace string
	pod       string
	container string
	command   []string
	stdin     bool
	stderr    bool
	tty       bool
}

// newExecutor builds the pod-exec request and its SPDY executor. Both the
// session and the sftp handlers go through here so the request shape stays
// in one place.
func newExecutor(clients *execClients, p execParams) (remotecommand.Executor, error) {
	request := clients.clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(p.pod).
		Namespace(p.namespace).
		SubResource("exec").
		Param("container", p.container).
		Param("stdout", "true").
		Param("stdin", strconv.FormatBool(p.stdin)).
		Param("stderr", strconv.FormatBool(p.stderr)).
		Param("tty", strconv.FormatBool(p.tty))
	for _, arg := range p.command {
		request = request.Param("command", arg)
	}
	return remotecommand.NewSPDYExecutor(clients.restConfig, "POST", request.URL())
}
