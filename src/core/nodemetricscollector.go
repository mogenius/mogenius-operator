package core

import (
	"context"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/cpumonitor"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/networkmonitor"
	"mogenius-k8s-manager/src/rammonitor"
	"mogenius-k8s-manager/src/shutdown"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NodeMetricsCollector interface {
	Run()
	Link(statsDb ValkeyStatsDb)
	Orchestrate()
}

type nodeMetricsCollector struct {
	logger         *slog.Logger
	config         config.ConfigModule
	clientProvider k8sclient.K8sClientProvider
	statsDb        ValkeyStatsDb
	leaderElector  LeaderElector

	cpuMonitor     cpumonitor.CpuMonitor
	ramMonitor     rammonitor.RamMonitor
	networkMonitor networkmonitor.NetworkMonitor
}

func NewNodeMetricsCollector(
	logger *slog.Logger,
	configModule config.ConfigModule,
	clientProviderModule k8sclient.K8sClientProvider,
	cpuMonitor cpumonitor.CpuMonitor,
	ramMonitor rammonitor.RamMonitor,
	networkMonitor networkmonitor.NetworkMonitor,
) NodeMetricsCollector {
	self := &nodeMetricsCollector{}

	self.logger = logger
	self.config = configModule
	self.clientProvider = clientProviderModule
	self.cpuMonitor = cpuMonitor
	self.ramMonitor = ramMonitor
	self.networkMonitor = networkMonitor

	return self
}

func (self *nodeMetricsCollector) Link(statsDb ValkeyStatsDb) {
	assert.Assert(statsDb != nil)

	self.statsDb = statsDb
}

func (self *nodeMetricsCollector) Orchestrate() {
	enabled, err := strconv.ParseBool(self.config.Get("MO_ENABLE_TRAFFIC_COLLECTOR"))
	assert.Assert(err == nil, err)
	self.logger.Info("node metrics collector configuration", "enabled", enabled)
	if !enabled {
		return
	}

	if self.clientProvider.RunsInCluster() {
		// setup daemonset
		self.leaderElector.OnLeading(func() {
			// check if daemonset exists
			// -> create if it doesnt
			// check if daemonset exists
			daemonSetName := "mogenius-k8s-manager-nodemetrics"
			namespace := self.config.Get("MO_OWN_NAMESPACE")

			clientset := self.clientProvider.K8sClientSet()
			if clientset == nil {
				self.logger.Error("failed to get Kubernetes clientset", "error", err)
				return
			}

			ownDeployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), "mogenius-k8s-manager", metav1.GetOptions{})
			if err != nil {
				self.logger.Error("failed to get own deployment for image name determination", "error", err)
				return
			}

			daemonSet, err := clientset.AppsV1().DaemonSets(namespace).Get(context.TODO(), daemonSetName, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					// DaemonSet does not exist, create it
					daemonSetSpec := &appsv1.DaemonSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      daemonSetName,
							Namespace: namespace,
						},
						Spec: appsv1.DaemonSetSpec{
							Selector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": daemonSetName},
							},
							Template: corev1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"app": daemonSetName},
								},
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  daemonSetName,
											Image: ownDeployment.Spec.Template.Spec.Containers[0].Image,
											Args:  []string{"nodemetrics"},
											Env: []corev1.EnvVar{
												{
													Name: "OWN_NAMESPACE",
													ValueFrom: &corev1.EnvVarSource{
														FieldRef: &corev1.ObjectFieldSelector{
															APIVersion: "v1",
															FieldPath:  "metadata.namespace",
														},
													},
												},
												{
													Name: "OWN_NODE_NAME",
													ValueFrom: &corev1.EnvVarSource{
														FieldRef: &corev1.ObjectFieldSelector{
															APIVersion: "v1",
															FieldPath:  "spec.nodeName",
														},
													},
												},
												{
													Name: "OWN_POD_NAME",
													ValueFrom: &corev1.EnvVarSource{
														FieldRef: &corev1.ObjectFieldSelector{
															APIVersion: "v1",
															FieldPath:  "metadata.name",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					}
					_, err := clientset.AppsV1().DaemonSets(namespace).Create(context.TODO(), daemonSetSpec, metav1.CreateOptions{})
					if err != nil {
						self.logger.Error("failed to create DaemonSet for node-metrics", "error", err)
						return
					}
					self.logger.Info("DaemonSet for node-metrics created successfully", "name", daemonSetName)
				} else {
					self.logger.Error("failed to get DaemonSet for node-metrics", "error", err)
					return
				}
			} else {
				self.logger.Info("DaemonSet for node-metrics already exists", "name", daemonSet.Name)
			}
		})
	} else {
		go func() {
			bin, err := os.Executable()
			assert.Assert(err == nil, "failed to get current executable path", err)

			nodemetrics := exec.Command(bin, "nodemetrics")
			outputBytes, err := nodemetrics.Output()
			if err != nil {
				// only print the last few lines to hopefully capture error messages
				output := string(outputBytes)
				outputLines := strings.Split(output, "\n")
				lastLinesStart := max(len(outputLines)-11, 0)
				lastLines := strings.Join(outputLines[lastLinesStart:], "\n")
				self.logger.Error("failed to run nodemetrics locally", "output", lastLines, "error", err)
				shutdown.SendShutdownSignal(true)
				select {}
			}
		}()
	}
}

func (self *nodeMetricsCollector) Run() {
	assert.Assert(self.logger != nil)
	assert.Assert(self.config != nil)
	assert.Assert(self.clientProvider != nil)
	assert.Assert(self.statsDb != nil)
	assert.Assert(self.cpuMonitor != nil)
	assert.Assert(self.ramMonitor != nil)
	assert.Assert(self.networkMonitor != nil)

	nodeName := self.config.Get("OWN_NODE_NAME")
	if !self.clientProvider.RunsInCluster() {
		nodeName = "local"
	}
	assert.Assert(nodeName != "")

	// network monitor
	go func() {
		self.networkMonitor.Run()
		go func() {
			for {
				metrics := self.networkMonitor.GetPodNetworkUsage()
				self.statsDb.AddInterfaceStatsToDb(metrics)
				time.Sleep(30 * time.Second)
			}
		}()
		go func() {
			for {
				metrics := self.networkMonitor.GetPodNetworkUsage()
				err := self.statsDb.AddNodeTrafficMetricsToDb(nodeName, metrics)
				if err != nil {
					self.logger.Error("failed to add node traffic metrics", "error", err)
				}
				time.Sleep(1 * time.Second)
			}
		}()
	}()

	// cpu usage
	go func() {
		for {
			metrics := self.cpuMonitor.CpuUsage()
			err := self.statsDb.AddNodeCpuMetricsToDb(nodeName, metrics)
			if err != nil {
				self.logger.Error("failed to add node cpu metrics", "error", err)
			}
			time.Sleep(1 * time.Second)
		}
	}()

	// ram usage
	go func() {
		for {
			metrics := self.ramMonitor.RamUsage()
			err := self.statsDb.AddNodeRamMetricsToDb(nodeName, metrics)
			if err != nil {
				self.logger.Error("failed to add node ram metrics", "error", err)
			}
			time.Sleep(1 * time.Second)
		}
	}()
}
