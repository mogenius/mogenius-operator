package core

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/cpumonitor"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/networkmonitor"
	"mogenius-k8s-manager/src/rammonitor"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type NodeMetricsCollector interface {
	// Run the nodemetrics collector locally.
	Run()
	Link(statsDb ValkeyStatsDb, leaderElector LeaderElector)
	// Manage instances of nodemetrics collector.
	// Either create the required DaemonSet or handle execution locally.
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

func (self *nodeMetricsCollector) Link(statsDb ValkeyStatsDb, leaderElector LeaderElector) {
	assert.Assert(statsDb != nil)
	assert.Assert(leaderElector != nil)

	self.statsDb = statsDb
	self.leaderElector = leaderElector
}

func (self *nodeMetricsCollector) Orchestrate() {
	trafficCollectorEnabled, err := strconv.ParseBool(self.config.Get("MO_ENABLE_TRAFFIC_COLLECTOR"))
	assert.Assert(err == nil, err)

	ownDeploymentName := self.config.Get("OWN_DEPLOYMENT_NAME")
	assert.Assert(ownDeploymentName != "")

	namespace := self.config.Get("MO_OWN_NAMESPACE")
	assert.Assert("MO_OWN_NAMESPACE" != "")

	daemonSetName := fmt.Sprintf("%s-nodemetrics", ownDeploymentName)

	if runtime.GOOS == "darwin" {
		self.logger.Error("SKIPPING node metrics collector setup on macOS", "reason", "not supported on macOS")
		return
	}

	self.logger.Info("node metrics collector configuration", "enabled", trafficCollectorEnabled)
	if !trafficCollectorEnabled {
		self.deleteDaemonSet(namespace, daemonSetName)
		return
	}

	if self.clientProvider.RunsInCluster() {
		self.leaderElector.OnLeading(func() {
			clientset := self.clientProvider.K8sClientSet()
			if clientset == nil {
				self.logger.Error("failed to get Kubernetes clientset", "error", err)
				return
			}

			ownDeployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), ownDeploymentName, metav1.GetOptions{})
			if err != nil {
				self.logger.Error("failed to get own deployment for image name determination", "error", err)
				return
			}

			ownerReference, err := utils.GetOwnDeploymentOwnerReference(clientset, self.config)
			if err != nil {
				self.logger.Error("failed to get own deployment owner reference", "error", err)
			}

			daemonSet, err := clientset.AppsV1().DaemonSets(namespace).Get(context.TODO(), daemonSetName, metav1.GetOptions{})
			daemonSetSpec := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:            daemonSetName,
					Namespace:       namespace,
					OwnerReferences: ownerReference,
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
									Command: []string{
										"dumb-init",
										"--",
										"mogenius-k8s-manager",
										"nodemetrics",
										"--metrics-rate",
										"2000",
										"--network-device-poll-rate",
										"1000",
									},
									Env: ownDeployment.Spec.Template.Spec.Containers[0].Env,
									SecurityContext: &corev1.SecurityContext{
										Capabilities: &corev1.Capabilities{
											Add: []corev1.Capability{
												"NET_RAW",
												"NET_ADMIN",
												"SYS_ADMIN",
												"SYS_PTRACE",
												"DAC_OVERRIDE",
												"SYS_RESOURCE",
											},
										},
										Privileged:             ptr.To(true),
										ReadOnlyRootFilesystem: ptr.To(true),
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											MountPath: self.config.Get("MO_HOST_PROC_PATH"),
											Name:      "proc",
											ReadOnly:  true,
										},
										{
											MountPath: "/hostcni",
											Name:      "cni",
											ReadOnly:  true,
										},
										{
											MountPath: "/sys",
											Name:      "sys",
											ReadOnly:  true,
										},
									},
								},
							},
							DNSPolicy:          corev1.DNSClusterFirstWithHostNet,
							HostNetwork:        true,
							ServiceAccountName: "mogenius-operator-service-account-app",
							Volumes: []corev1.Volume{
								{
									Name: "proc",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/proc",
										},
									},
								},
								{
									Name: "cni",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/etc/cni/net.d",
										},
									},
								},
								{
									Name: "sys",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/sys",
										},
									},
								},
							},
						},
					},
				},
			}
			if err != nil {
				if k8sErrors.IsNotFound(err) {
					// CREATE
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
				// UPDATE
				_, err := clientset.AppsV1().DaemonSets(namespace).Update(context.TODO(), daemonSetSpec, metav1.UpdateOptions{})
				if err != nil {
					self.logger.Error("failed to update DaemonSet for node-metrics", "error", err)
					return
				}
				self.logger.Info("DaemonSet for node-metrics already exists. Updating DaemonSet because of possible changes", "name", daemonSet.Name)
			}
		})
	} else {
		go func() {
			self.deleteDaemonSet(namespace, daemonSetName)

			bin, err := os.Executable()
			assert.Assert(err == nil, "failed to get current executable path", err)

			nodemetrics := exec.Command(bin, "nodemetrics")

			// This buffer is allocated both for stdout and stderr.
			// Since this only happens in local development we dont have to care for a few megabytes of statically allocated memory.
			bufSize := 5 * 1024 * 1024 // 5MiB
			stdoutPipe, err := nodemetrics.StdoutPipe()
			assert.Assert(err == nil, "reading stdout of this child process has to work", err)
			stderrPipe, err := nodemetrics.StderrPipe()
			assert.Assert(err == nil, "reading stderr of this child process has to work", err)

			go func() {
				scanner := bufio.NewScanner(stdoutPipe)
				scanner.Buffer(make([]byte, bufSize), bufSize)
				for scanner.Scan() {
					output := string(scanner.Bytes())
					fmt.Fprintf(os.Stderr, "node-metrics %s | %s\n", "stdout", output)
				}
			}()

			go func() {
				scanner := bufio.NewScanner(stderrPipe)
				scanner.Buffer(make([]byte, bufSize), bufSize)
				for scanner.Scan() {
					output := scanner.Bytes()
					fmt.Fprintf(os.Stderr, "| node-metrics %s | %s\n", "stderr", output)
				}
			}()

			err = nodemetrics.Start()
			if err != nil {
				self.logger.Error("failed to start node-metrics", "error", err)
				shutdown.SendShutdownSignal(true)
				select {}
			}

			err = nodemetrics.Wait()
			if err != nil {
				self.logger.Error("failed to wait for node-metrics", "error", err)
				shutdown.SendShutdownSignal(true)
				select {}
			}
		}()
	}
}

func (self *nodeMetricsCollector) deleteDaemonSet(namespace string, daemonSetName string) {
	clientset := self.clientProvider.K8sClientSet()
	assert.Assert(clientset != nil, "failed to get Kubernetes clientset")

	_, err := clientset.AppsV1().DaemonSets(namespace).Get(context.TODO(), daemonSetName, metav1.GetOptions{})
	if err == nil {
		err := clientset.AppsV1().DaemonSets(namespace).Delete(context.TODO(), daemonSetName, metav1.DeleteOptions{})
		if err != nil {
			self.logger.Error("failed to delete node-metrics daemonset", "error", err)
		} else {
			self.logger.Info("node-metrics daemonset deleted successfully")
		}
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

	// node-stats monitor
	go func() {
		machinestats := structs.MachineStats{
			BtfSupport: self.networkMonitor.BtfAvailable(),
		}
		for {
			err := self.statsDb.AddMachineStatsToDb(nodeName, machinestats)
			if err != nil {
				self.logger.Warn("failed to write machine stats for node", "node", nodeName, "error", err)
			}
			time.Sleep(1 * time.Minute)
		}
	}()

	// network monitor
	go func() {
		self.networkMonitor.Run()
		go func() {
			for {
				metrics := self.networkMonitor.GetPodNetworkUsage()
				self.statsDb.AddInterfaceStatsToDb(metrics)
				time.Sleep(60 * time.Second)
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
		go func() {
			for {
				status := self.networkMonitor.Snoopy().Status()
				err := self.statsDb.AddSnoopyStatusToDb(nodeName, status)
				if err != nil {
					self.logger.Error("failed to store snoopy status", "error", err)
				}
				time.Sleep(1 * time.Second)
			}
		}()
	}()

	// cpu usage
	go func() {
		go func() {
			for {
				metrics := self.cpuMonitor.CpuUsageGlobal()
				err := self.statsDb.AddNodeCpuMetricsToDb(nodeName, metrics)
				if err != nil {
					self.logger.Error("failed to add node cpu metrics", "error", err)
				}
				time.Sleep(1 * time.Second)
			}
		}()
		go func() {
			for {
				metrics := self.cpuMonitor.CpuUsageProcesses()
				_ = metrics
				// self.logger.Info("collected process cpu info", "metrics", metrics)
				time.Sleep(1 * time.Second)
			}
		}()
	}()

	// ram usage
	go func() {
		go func() {
			for {
				metrics := self.ramMonitor.RamUsageGlobal()
				err := self.statsDb.AddNodeRamMetricsToDb(nodeName, metrics)
				if err != nil {
					self.logger.Error("failed to add node ram metrics", "error", err)
				}
				time.Sleep(1 * time.Second)
			}
		}()

		go func() {
			for {
				metrics := self.ramMonitor.RamUsageProcesses()
				_ = metrics
				// self.logger.Info("collected process memory info", "metrics", metrics)
				time.Sleep(1 * time.Second)
			}
		}()
	}()
}
