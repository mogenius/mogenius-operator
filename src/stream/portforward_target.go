package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mogenius-operator/src/sandboxactivity"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// How long a new kube-apiserver port-forward may take to become ready.
const pfForwardReadyTimeout = 30 * time.Second

// Backoff between attempts to rebuild a lost port-forward while the tunnel
// lives: the workload may have no running pod for a while during a rollout or
// a scale to zero.
const (
	pfRebuildMinBackoff = 1 * time.Second
	pfRebuildMaxBackoff = 30 * time.Second
)

// How long opening a sub-connection waits for a rebuilt port-forward after its
// dial failed, before it reports the target as unreachable.
const pfRebuildDialWait = 5 * time.Second

// How often a tunnel checks that the pod its port-forward reaches still runs.
// The forward does not always end when its pod goes away: the connection to
// the kube-apiserver can stay up while every stream to the deleted pod fails,
// so the local port keeps accepting and only the bytes never arrive.
const pfPodCheckInterval = 5 * time.Second

// errNoRunningPod: the tunnel's port-forward is gone and no pod of the target
// came up in time to rebuild it (a rollout still in progress, a scale to zero).
var errNoRunningPod = errors.New("no running pod for the tunnel target")

// podForward is one kube-apiserver port-forward to one pod.
type podForward struct {
	podName  string
	dialAddr string
	stop     chan struct{}
	stopOnce sync.Once
	// lost is closed once the forward has ended — the pod went away, the
	// connection to the kube-apiserver dropped, or it was stopped.
	lost <-chan struct{}
	// err holds why the forward ended; valid only after lost is closed.
	err error
}

func (f *podForward) close() {
	f.stopOnce.Do(func() { close(f.stop) })
}

// startKubePodForward forwards a free local port to remotePort on the pod and
// returns once the forward is ready.
func startKubePodForward(namespace, podName string, remotePort int) (*podForward, error) {
	localListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("local listener: %w", err)
	}
	localPort := localListener.Addr().(*net.TCPAddr).Port
	_ = localListener.Close()

	pfURL, err := url.Parse(fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/portforward",
		pfRestConfig.Host, namespace, podName))
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	transport, upgrader, err := spdy.RoundTripperFor(pfRestConfig)
	if err != nil {
		return nil, fmt.Errorf("SPDY transport: %w", err)
	}
	spdyDialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, pfURL)

	lost := make(chan struct{})
	forward := &podForward{
		podName:  podName,
		dialAddr: fmt.Sprintf("127.0.0.1:%d", localPort),
		stop:     make(chan struct{}),
		lost:     lost,
	}
	readyChan := make(chan struct{})

	fw, err := portforward.New(spdyDialer, []string{fmt.Sprintf("%d:%d", localPort, remotePort)},
		forward.stop, readyChan, io.Discard, io.Discard)
	if err != nil {
		return nil, fmt.Errorf("portforward create: %w", err)
	}

	go func() {
		// ForwardPorts returns when the forward is stopped or the stream to
		// the pod is lost (pod deleted, kube-apiserver connection dropped).
		forward.err = fw.ForwardPorts()
		close(lost)
	}()

	select {
	case <-readyChan:
		return forward, nil
	case <-lost:
		if forward.err == nil {
			return nil, fmt.Errorf("port-forward to pod %q ended before it was ready", podName)
		}
		return nil, forward.err
	case <-time.After(pfForwardReadyTimeout):
		forward.close()
		return nil, fmt.Errorf("port-forward to pod %q not ready after %s", podName, pfForwardReadyTimeout)
	}
}

// podForwarder keeps a port-forward to whichever pod currently backs a tunnel
// target. The target is a workload, not a pod: when its pod goes away (a
// rollout, an eviction, a scale to zero and back), the forward is rebuilt on
// the pod that backs the workload now, so the tunnel keeps working without
// being created again.
type podForwarder struct {
	logger  *slog.Logger
	resolve func() (string, error)
	start   func(podName string) (*podForward, error)
	// alive reports whether the pod still runs; an error means "unknown" and
	// never triggers a rebuild on its own.
	alive func(podName string) (bool, error)
	hold  *podHold

	minBackoff      time.Duration
	maxBackoff      time.Duration
	rebuildDialWait time.Duration
	checkInterval   time.Duration

	mu      sync.Mutex
	current *podForward
	// closed is set by close: a forward that replace finishes starting after
	// the tunnel closed must be stopped instead of installed, or its kube
	// port-forward and the sandbox hold it re-takes would leak.
	closed bool
	// changed is closed and replaced whenever current changes, so a waiting
	// dial can wake up.
	changed chan struct{}
	// rebuild asks run to replace the current forward now; capacity 1, so
	// any number of failed dials collapse into one rebuild.
	rebuild chan struct{}
	// check asks run to look at the pod now rather than at the next tick;
	// capacity 1 as well.
	check chan struct{}
}

func newPodForwarder(
	logger *slog.Logger,
	resolve func() (string, error),
	start func(string) (*podForward, error),
	alive func(string) (bool, error),
	hold *podHold,
) *podForwarder {
	return &podForwarder{
		logger:          logger,
		resolve:         resolve,
		start:           start,
		alive:           alive,
		hold:            hold,
		minBackoff:      pfRebuildMinBackoff,
		maxBackoff:      pfRebuildMaxBackoff,
		rebuildDialWait: pfRebuildDialWait,
		checkInterval:   pfPodCheckInterval,
		changed:         make(chan struct{}),
		rebuild:         make(chan struct{}, 1),
		check:           make(chan struct{}, 1),
	}
}

// startOn opens the first forward, on the pod resolved when the tunnel opened.
func (f *podForwarder) startOn(podName string) error {
	forward, err := f.start(podName)
	if err != nil {
		return err
	}
	f.set(forward)
	return nil
}

func (f *podForwarder) set(forward *podForward) {
	f.mu.Lock()
	if f.closed && forward != nil {
		f.mu.Unlock()
		forward.close()
		return
	}
	defer f.mu.Unlock()
	f.current = forward
	if forward != nil {
		f.hold.set(forward.podName)
	}
	close(f.changed)
	f.changed = make(chan struct{})
}

// run watches the current forward until done closes, and rebuilds it when it
// is lost, a dial through it failed, or its pod no longer runs.
func (f *podForwarder) run(done <-chan struct{}) {
	ticker := time.NewTicker(f.checkInterval)
	defer ticker.Stop()

	for {
		f.mu.Lock()
		current := f.current
		f.mu.Unlock()

		var lost <-chan struct{}
		if current != nil {
			lost = current.lost
		}

		select {
		case <-done:
			return
		case <-lost:
			f.logger.Warn("Port-forward lost, re-resolving target", "pod", current.podName, "error", current.err)
		case <-f.rebuild:
			f.logger.Warn("Dial through port-forward failed, re-resolving target", "pod", podNameOf(current))
		case <-ticker.C:
			if f.podRuns(current) {
				continue
			}
		case <-f.check:
			if f.podRuns(current) {
				continue
			}
		}

		f.replace(current, done)
	}
}

// podRuns reports whether the forward's pod still runs. Unknown counts as
// running: a failed lookup must not tear down a working tunnel.
func (f *podForwarder) podRuns(current *podForward) bool {
	if current == nil {
		return true
	}
	running, err := f.alive(current.podName)
	if err != nil {
		f.logger.Debug("Could not check the tunnel's pod", "pod", current.podName, "error", err)
		return true
	}
	if !running {
		f.logger.Warn("Tunnel's pod is gone, re-resolving target", "pod", current.podName)
	}
	return running
}

// replace stops the old forward and retries resolve + start with backoff
// until a new forward is up or done closes.
func (f *podForwarder) replace(old *podForward, done <-chan struct{}) {
	f.set(nil)
	if old != nil {
		old.close()
	}

	backoff := f.minBackoff
	for {
		podName, err := f.resolve()
		var forward *podForward
		if err == nil {
			forward, err = f.start(podName)
		}
		if err == nil {
			f.set(forward)
			// Dials that failed while rebuilding are answered by this forward.
			select {
			case <-f.rebuild:
			default:
			}
			f.logger.Info("Port-forward re-established", "pod", podName)
			return
		}

		f.logger.Warn("Port-forward not re-established yet", "error", err, "retryIn", backoff)
		select {
		case <-done:
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, f.maxBackoff)
	}
}

// forwardOtherThan returns the current forward if it is not failed (nil for
// any), waiting up to wait for one to come up. Compared by identity, not by
// address: a rebuilt forward may well get the local port the old one freed.
func (f *podForwarder) forwardOtherThan(failed *podForward, wait time.Duration, done <-chan struct{}) (*podForward, bool) {
	deadline := time.After(wait)
	for {
		f.mu.Lock()
		current, changed := f.current, f.changed
		f.mu.Unlock()

		if current != nil && current != failed {
			return current, true
		}
		select {
		case <-changed:
		case <-deadline:
			return nil, false
		case <-done:
			return nil, false
		}
	}
}

// requestCheck asks run to look at the pod now. Used when a sub-connection
// ended without a single byte from the pod — a sign the pod may be gone, but
// also what a target that closes idle connections does, so it only checks and
// never rebuilds blindly (a rebuild cuts every other open connection).
func (f *podForwarder) requestCheck() {
	select {
	case f.check <- struct{}{}:
	default:
	}
}

// requestRebuild asks run to replace the current forward; a rebuild already
// asked for absorbs it.
func (f *podForwarder) requestRebuild() {
	select {
	case f.rebuild <- struct{}{}:
	default:
	}
}

// dial connects through the current forward. A failed dial means the local
// end of the forward is gone, so it asks for a rebuild and tries once more on
// the new forward.
func (f *podForwarder) dial(done <-chan struct{}, dialTimeout time.Duration) (net.Conn, error) {
	failed, ok := f.forwardOtherThan(nil, 0, done)
	if ok {
		conn, err := net.DialTimeout("tcp", failed.dialAddr, dialTimeout)
		if err == nil {
			return conn, nil
		}
		f.logger.Info("Dial through port-forward failed", "pod", failed.podName, "error", err)
	}

	f.requestRebuild()
	next, ok := f.forwardOtherThan(failed, f.rebuildDialWait, done)
	if !ok {
		return nil, errNoRunningPod
	}
	return net.DialTimeout("tcp", next.dialAddr, dialTimeout)
}

// close stops the current forward and releases the pod's activity hold.
func (f *podForwarder) close() {
	f.mu.Lock()
	current := f.current
	f.current = nil
	f.closed = true
	f.mu.Unlock()
	if current != nil {
		current.close()
	}
	f.hold.release()
}

func podNameOf(forward *podForward) string {
	if forward == nil {
		return ""
	}
	return forward.podName
}

// podHold keeps the sandbox-activity hold on the pod a tunnel currently
// reaches, moving it along when the tunnel moves to another pod.
type podHold struct {
	namespace string
	hold      func(namespace, pod string) func()

	mu        sync.Mutex
	pod       string
	releaseFn func()
}

func newPodHold(namespace string) *podHold {
	return &podHold{namespace: namespace, hold: sandboxactivity.Hold}
}

func (h *podHold) set(pod string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if pod == h.pod {
		return
	}
	if h.releaseFn != nil {
		h.releaseFn()
	}
	h.pod = pod
	h.releaseFn = h.hold(h.namespace, pod)
}

func (h *podHold) release() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.releaseFn != nil {
		h.releaseFn()
	}
	h.pod, h.releaseFn = "", nil
}

// podAlive reports whether the pod exists, is not being deleted and runs.
func podAlive(clientset k8s.Interface, namespace, podName string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return pod.DeletionTimestamp == nil && pod.Status.Phase == corev1.PodRunning, nil
}

// pickRunningPod chooses the pod a tunnel should reach: never one that is
// being deleted — it still reports phase Running while it terminates, so a
// rollout would otherwise hand the tunnel the pod that is about to vanish —
// and a ready one before one that is still starting.
func pickRunningPod(pods []corev1.Pod) (string, bool) {
	fallback := ""
	for _, pod := range pods {
		if pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning {
			continue
		}
		if podIsReady(pod) {
			return pod.Name, true
		}
		if fallback == "" {
			fallback = pod.Name
		}
	}
	return fallback, fallback != ""
}

func podIsReady(pod corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}
