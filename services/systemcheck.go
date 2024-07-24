package services

import (
	"fmt"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"net/netip"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	punqDtos "github.com/mogenius/punq/dtos"
	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	"helm.sh/helm/v3/pkg/release"
)

type SystemCheckEntry struct {
	CheckName          string         `json:"checkName"`
	HelmStatus         release.Status `json:"helmStatus"`
	IsRunning          bool           `json:"isRunning"`
	SuccessMessage     string         `json:"successMessage"`
	ErrorMessage       *string        `json:"errorMessage"`
	SolutionMessage    string         `json:"solutionMessage"`
	Description        string         `json:"description"`
	InstallPattern     string         `json:"installPattern"`
	UpgradePattern     string         `json:"upgradePattern"`
	UninstallPattern   string         `json:"uninstallPattern"`
	IsRequired         bool           `json:"isRequired"`
	WantsToBeInstalled bool           `json:"wantsToBeInstalled"`
	VersionInstalled   string         `json:"versionInstalled"`
	VersionAvailable   string         `json:"versionAvailable"`
	ProcessTimeInMs    int64          `json:"processTimeInMs"`
}

// sort.Interface for []SystemCheckEntry based on the CheckName field.
type ByCheckName []SystemCheckEntry

func (a ByCheckName) Len() int           { return len(a) }
func (a ByCheckName) Less(i, j int) bool { return a[i].CheckName < a[j].CheckName }
func (a ByCheckName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

var syscheckmutex sync.Mutex

type SystemCheckResponse struct {
	TerminalString string             `json:"terminalString"`
	Entries        []SystemCheckEntry `json:"entries"`
}

func CreateSystemCheckEntry(checkName string, isRunning bool, successMessage string, solutionMsg string, err error, description string, isRequired bool, wantsToBeInstalled bool, versionInstalled string, versionAvailable string) SystemCheckEntry {

	var errMsg *string
	if err != nil {
		errMsgStr := err.Error()
		errMsg = &errMsgStr
	}

	return SystemCheckEntry{
		IsRunning:          isRunning,
		CheckName:          checkName,
		SuccessMessage:     successMessage,
		ErrorMessage:       errMsg,
		SolutionMessage:    solutionMsg,
		Description:        description,
		IsRequired:         isRequired,
		WantsToBeInstalled: wantsToBeInstalled,
		VersionInstalled:   versionInstalled,
		VersionAvailable:   versionAvailable,
	}
}

// SysCheckExec measures the execution time of a function
func SysCheckExec(name string, wg *sync.WaitGroup, entries *[]SystemCheckEntry, fn func() SystemCheckEntry) {
	defer wg.Done()
	start := time.Now()
	entry := fn()
	duration := time.Since(start)
	entry.ProcessTimeInMs = duration.Milliseconds()
	syscheckmutex.Lock()
	*entries = append(*entries, entry)
	syscheckmutex.Unlock()
}

func SystemCheck() SystemCheckResponse {
	var wg sync.WaitGroup
	entries := []SystemCheckEntry{}

	// check internet access
	wg.Add(1)
	go SysCheckExec("CheckInternetAccess", &wg, &entries, func() SystemCheckEntry {
		inetResult, inetErr := punqUtils.CheckInternetAccess()
		return CreateSystemCheckEntry(
			"Internet Access",
			inetResult,
			"Internet access works.",
			"Please check your internet connection.",
			inetErr,
			"",
			true,
			false,
			"",
			"")
	})

	// check for kubectl
	wg.Add(1)
	go SysCheckExec("CheckKubectl", &wg, &entries, func() SystemCheckEntry {
		kubectlResult, kubectlOutput, kubectlErr := punqUtils.IsKubectlInstalled()
		return CreateSystemCheckEntry("kubectl",
			kubectlResult,
			kubectlOutput,
			"Plase install kubectl (https://kubernetes.io/docs/tasks/tools/) on your system to proceed.",
			kubectlErr,
			"",
			true,
			false,
			"",
			"")
	})

	// check kubernetes version
	wg.Add(1)
	go SysCheckExec("CheckKubectlAndKubernetesVersion", &wg, &entries, func() SystemCheckEntry {
		kubernetesVersion := punq.KubernetesVersion(nil)
		kubernetesVersionResult := kubernetesVersion != nil
		return CreateSystemCheckEntry("Kubernetes Version",
			kubernetesVersionResult,
			fmt.Sprintf("Version: %s\nPlatform: %s", kubernetesVersion.String(), kubernetesVersion.Platform),
			"Cannot determine version of kubernetes.",
			nil,
			"",
			true,
			false,
			kubernetesVersion.String(), "")
	})

	// check for ingresscontroller
	wg.Add(1)
	go SysCheckExec("CheckIngressController", &wg, &entries, func() SystemCheckEntry {
		ingressType, ingressTypeErr := punq.DetermineIngressControllerType(nil)
		ingrEntry := CreateSystemCheckEntry(
			"Ingress Controller",
			ingressTypeErr == nil,
			ingressType.String(),
			"Cannot determin ingress controller type.",
			ingressTypeErr,
			"Installs a traefik ingress controller to handle traffic from outside the cluster and more.",
			false,
			true,
			"",
			"")
		ingrEntry.InstallPattern = structs.PAT_INSTALL_INGRESS_CONTROLLER_TREAFIK
		ingrEntry.UninstallPattern = structs.PAT_UNINSTALL_INGRESS_CONTROLLER_TREAFIK
		ingrEntry.UpgradePattern = "" // structs.PAT_UPGRADE_INGRESS_CONTROLLER_TREAFIK
		ingrEntry.VersionAvailable = getMostCurrentHelmChartVersion(IngressControllerTraefikHelmIndex, utils.HelmReleaseNameTraefik)
		ingrEntry.HelmStatus = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameTraefik)
		return ingrEntry
	})

	// check for metrics server
	wg.Add(1)
	go SysCheckExec("CheckMetricsServer", &wg, &entries, func() SystemCheckEntry {
		metricsResult, metricsVersion, metricsErr := punq.IsMetricsServerAvailable(nil)
		metricsEntry := CreateSystemCheckEntry(
			"Metrics Server",
			metricsResult,
			metricsVersion,
			"kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml\nNote: Running docker-desktop? Please add '- --kubelet-insecure-tls' to the args sction in the deployment of metrics-server.",
			metricsErr,
			"Maintained by Kubernetes-SIGs, handles metrics for your cluster.",
			true,
			true,
			metricsVersion,
			"")
		metricsEntry.InstallPattern = structs.PAT_INSTALL_METRICS_SERVER
		metricsEntry.UninstallPattern = structs.PAT_UNINSTALL_METRICS_SERVER
		metricsEntry.UpgradePattern = "" // structs.PAT_UPGRADE_METRICS_SERVER
		metricsEntry.VersionAvailable = getMostCurrentHelmChartVersion(MetricsHelmIndex, utils.HelmReleaseNameMetricsServer)
		metricsEntry.HelmStatus = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameMetricsServer)
		return metricsEntry
	})

	// check for helm
	wg.Add(1)
	go SysCheckExec("CheckHelm", &wg, &entries, func() SystemCheckEntry {
		helmResult, helmOutput, helmErr := punqUtils.IsHelmInstalled()
		return CreateSystemCheckEntry(
			"Helm",
			helmResult,
			helmOutput,
			"Plase install helm (https://helm.sh/docs/intro/install/) on your system to proceed.",
			helmErr,
			"",
			true,
			false,
			helmOutput,
			"")
	})

	// check cluster provider
	wg.Add(1)
	go SysCheckExec("CheckClusterProvider", &wg, &entries, func() SystemCheckEntry {
		clusterProvOutput, clusterProvErr := punq.GuessClusterProvider(nil)
		return CreateSystemCheckEntry(
			"Cluster Provider",
			clusterProvErr == nil,
			string(clusterProvOutput),
			"We could not determine the provider of this cluster.",
			clusterProvErr,
			"",
			false,
			false,
			"",
			"")
	})

	// API Versions
	wg.Add(1)
	go SysCheckExec("CheckApiVersions", &wg, &entries, func() SystemCheckEntry {
		apiVerResult, apiVerErr := punq.ApiVersions(nil)
		apiVersStr := ""
		for _, entry := range apiVerResult {
			apiVersStr += fmt.Sprintf("%s\n", entry)
		}
		apiVersStr = strings.TrimRight(apiVersStr, "\n\r")
		return CreateSystemCheckEntry("API Versions",
			len(apiVerResult) > 0,
			apiVersStr,
			"Kubernetes Server Prefered versions could not be determined.",
			apiVerErr,
			"",
			true,
			false,
			"",
			"")
	})

	// check country of cluster
	wg.Add(1)
	go SysCheckExec("CheckClusterCountry", &wg, &entries, func() SystemCheckEntry {
		countryResult, countryErr := punqUtils.GuessClusterCountry()
		countryName := ""
		if countryResult != nil {
			countryName = countryResult.Name
		}
		return CreateSystemCheckEntry(
			"Cluster Country",
			true,
			countryName,
			"We could not determine the location of the cluster.",
			countryErr,
			"",
			false,
			false,
			"",
			"")
	})

	// check for loadbalancer ips
	wg.Add(1)
	go SysCheckExec("CheckLoadBalancerIps", &wg, &entries, func() SystemCheckEntry {
		lbName := "LoadBalancer IPs/Hosts"
		loadbalancerIps := punq.GetClusterExternalIps(nil)
		var lbIpsErr error
		if len(loadbalancerIps) == 0 {
			lbIpsErr = fmt.Errorf("No external IPs/Hosts.\nMaybe you don't have TREAFIK or NGINX ingress controller installed.")
		}
		return CreateSystemCheckEntry(
			lbName,
			len(loadbalancerIps) > 0,
			strings.Join(loadbalancerIps, ", "),
			"Please check your ingress controller configuration. You need to have an ingress controller installed to have external IPs/Hosts.",
			lbIpsErr,
			"",
			false,
			false,
			"",
			"")
	})

	// check for docker
	wg.Add(1)
	go SysCheckExec("CheckDocker", &wg, &entries, func() SystemCheckEntry {
		dockerResult, dockerOutput, dockerErr := IsDockerInstalled()
		return CreateSystemCheckEntry(
			"docker",
			dockerResult,
			dockerOutput,
			"",
			dockerErr,
			"",
			true,
			false,
			dockerOutput,
			"")
	})

	// check for cert-manager
	wg.Add(1)
	go SysCheckExec("CheckCertManager", &wg, &entries, func() SystemCheckEntry {
		certManagerVersion, certManagerInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameCertManager)
		certManagerMsg := fmt.Sprintf("%s (Version: %s) is installed.", utils.HelmReleaseNameCertManager, certManagerVersion)
		currentCertManagerVersion := getMostCurrentHelmChartVersion(CertManagerHelmIndex, utils.HelmReleaseNameCertManager)
		certMgrEntry := CreateSystemCheckEntry(
			utils.HelmReleaseNameCertManager,
			certManagerInstalledErr == nil,
			certManagerMsg,
			fmt.Sprintf("%s is not installed.\nTo create ssl certificates you need to install this component.", utils.HelmReleaseNameCertManager),
			certManagerInstalledErr,
			"Install the cert-manager to automatically issue Let's Encrypt certificates to your services.",
			false,
			true,
			certManagerVersion,
			currentCertManagerVersion)
		certMgrEntry.InstallPattern = structs.PAT_INSTALL_CERT_MANAGER
		certMgrEntry.UninstallPattern = structs.PAT_UNINSTALL_CERT_MANAGER
		certMgrEntry.UpgradePattern = "" // structs.PAT_UPGRADE_CERT_MANAGER
		certMgrEntry.HelmStatus = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameCertManager)
		return certMgrEntry
	})

	// check for clusterissuer
	wg.Add(1)
	go SysCheckExec("CheckClusterIssuer", &wg, &entries, func() SystemCheckEntry {
		_, clusterIssuerInstalledErr := punq.GetClusterIssuer(NameClusterIssuerResource, nil)
		clusterIssuerMsg := fmt.Sprintf("%s is installed.", NameClusterIssuerResource)
		clusterIssuerEntry := CreateSystemCheckEntry(
			utils.HelmReleaseNameClusterIssuer,
			clusterIssuerInstalledErr == nil,
			clusterIssuerMsg,
			fmt.Sprintf("%s is not installed.\nTo issue ssl certificates you need to install this component.", NameClusterIssuerResource),
			clusterIssuerInstalledErr,
			"Responsible for signing certificates.",
			false,
			true,
			"",
			"")
		clusterIssuerEntry.InstallPattern = structs.PAT_INSTALL_CLUSTER_ISSUER
		clusterIssuerEntry.UninstallPattern = structs.PAT_UNINSTALL_CLUSTER_ISSUER
		clusterIssuerEntry.HelmStatus = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameClusterIssuer)
		return clusterIssuerEntry
	})

	// check for trafficcollector
	wg.Add(1)
	go SysCheckExec("CheckTrafficCollector", &wg, &entries, func() SystemCheckEntry {
		trafficCollectorNewestVersion, err := getCurrentTrafficCollectorVersion()
		if err != nil {
			ServiceLogger.Errorf("getCurrentTrafficCollectorVersion Err: %s", err.Error())
		}
		trafficCollectorVersion, trafficCollectorInstalledErr := punq.IsDaemonSetInstalled(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameTrafficCollector)
		if trafficCollectorVersion == "" && trafficCollectorInstalledErr == nil {
			trafficCollectorVersion = "6.6.6" // flag local version without tag
		}
		trafficMsg := fmt.Sprintf("%s (Version: %s) is installed.", utils.HelmReleaseNameTrafficCollector, trafficCollectorVersion)
		trafficEntry := CreateSystemCheckEntry(
			utils.HelmReleaseNameTrafficCollector,
			trafficCollectorInstalledErr == nil, trafficMsg,
			fmt.Sprintf("%s is not installed.\nTo gather traffic information you need to install this component.", utils.HelmReleaseNameTrafficCollector),
			trafficCollectorInstalledErr,
			fmt.Sprintf("Collects and exposes detailed traffic data for your mogenius services for better monitoring. (Installed: %s | Available: %s)", trafficCollectorVersion, trafficCollectorNewestVersion),
			true,
			true,
			trafficCollectorVersion,
			trafficCollectorNewestVersion)
		trafficEntry.InstallPattern = structs.PAT_INSTALL_TRAFFIC_COLLECTOR
		trafficEntry.UninstallPattern = structs.PAT_UNINSTALL_TRAFFIC_COLLECTOR
		trafficEntry.UpgradePattern = structs.PAT_UPGRADE_TRAFFIC_COLLECTOR
		trafficEntry.HelmStatus = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameTrafficCollector)
		return trafficEntry
	})

	// check for podstatscollector
	wg.Add(1)
	go SysCheckExec("CheckPodStatsCollector", &wg, &entries, func() SystemCheckEntry {
		podstatsCollectorNewestVersion, err := getCurrentPodStatsCollectorVersion()
		if err != nil {
			ServiceLogger.Errorf("getCurrentPodStatsCollectorVersion Err: %s", err.Error())
		}
		podStatsCollectorVersion, podStatsCollectorInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNamePodStatsCollector)
		if podStatsCollectorVersion == "" && podStatsCollectorInstalledErr == nil {
			podStatsCollectorVersion = "6.6.6" // flag local version without tag
		}
		podStatsMsg := fmt.Sprintf("%s (Version: %s) is installed.", utils.HelmReleaseNamePodStatsCollector, podStatsCollectorVersion)

		podEntry := CreateSystemCheckEntry(
			utils.HelmReleaseNamePodStatsCollector,
			podStatsCollectorInstalledErr == nil,
			podStatsMsg,
			fmt.Sprintf("%s is not installed.\nTo gather pod/event information you need to install this component.", utils.HelmReleaseNamePodStatsCollector),
			podStatsCollectorInstalledErr,
			fmt.Sprintf("Collects and exposes status events of pods for services in mogenius. (Installed: %s | Available: %s)", podStatsCollectorVersion, podstatsCollectorNewestVersion),
			true,
			true,
			podStatsCollectorVersion,
			podstatsCollectorNewestVersion)
		podEntry.InstallPattern = structs.PAT_INSTALL_POD_STATS_COLLECTOR
		podEntry.UninstallPattern = structs.PAT_UNINSTALL_POD_STATS_COLLECTOR
		podEntry.UpgradePattern = structs.PAT_UPGRADE_PODSTATS_COLLECTOR
		podEntry.HelmStatus = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNamePodStatsCollector)
		return podEntry
	})

	// check for distribution registry
	wg.Add(1)
	go SysCheckExec("CheckDistributionRegistry", &wg, &entries, func() SystemCheckEntry {
		distributionRegistryName := "distribution-registry-docker-registry"
		distriRegistryVersion, distriRegistryInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, distributionRegistryName)
		distriRegistryMsg := fmt.Sprintf("%s (Version: %s) is installed.", distributionRegistryName, distriRegistryVersion)
		currentDistriRegistryVersion := getMostCurrentHelmChartVersion(ContainerRegistryHelmIndex, "docker-registry")
		distriEntry := CreateSystemCheckEntry(
			NameInternalContainerRegistry,
			distriRegistryInstalledErr == nil,
			distriRegistryMsg,
			fmt.Sprintf("%s is not installed.\nTo have a private container registry running inside your cluster, you need to install this component.", distributionRegistryName),
			distriRegistryInstalledErr,
			"A Docker-based container registry inside Kubernetes.",
			false,
			true,
			distriRegistryVersion,
			currentDistriRegistryVersion)
		distriEntry.InstallPattern = structs.PAT_INSTALL_CONTAINER_REGISTRY
		distriEntry.UninstallPattern = structs.PAT_UNINSTALL_CONTAINER_REGISTRY
		distriEntry.UpgradePattern = "" // structs.PAT_UPGRADE_CONTAINER_REGISTRY
		distriEntry.HelmStatus = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameDistributionRegistry)
		return distriEntry
	})

	// check for external secrets
	if utils.CONFIG.Misc.ExternalSecretsEnabled {
		wg.Add(1)
		go SysCheckExec("CheckExternalSecrets", &wg, &entries, func() SystemCheckEntry {
			externalSecretsName := "external-secrets"
			externalSecretsVersion, externalSecretsInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, externalSecretsName)
			externalSecretsMsg := fmt.Sprintf("%s (Version: %s) is installed.", externalSecretsName, externalSecretsVersion)

			currentExternalSecretsVersion := getMostCurrentHelmChartVersion(ExternalSecretsHelmIndex, "docker-registry")
			externalSecretsEntry := CreateSystemCheckEntry(
				NameExternalSecrets,
				externalSecretsInstalledErr == nil,
				externalSecretsMsg,
				fmt.Sprintf("%s is not installed.\nTo load secrets from 3rd party vaults (e.g. e.g. Hashicorp Vault, AWS KMS or Azure Key Vault), you need to install this component.", externalSecretsName),
				externalSecretsInstalledErr,
				"A Docker-based External Secrets loader inside Kubernetes that allows you to connect to e.g. Hashicorp Vault, AWS KMS or Azure Key Vault",
				false,
				false,
				externalSecretsVersion,
				currentExternalSecretsVersion)
			externalSecretsEntry.InstallPattern = structs.PAT_INSTALL_EXTERNAL_SECRETS
			externalSecretsEntry.UninstallPattern = structs.PAT_UNINSTALL_EXTERNAL_SECRETS
			externalSecretsEntry.UpgradePattern = "" // NONE?
			externalSecretsEntry.HelmStatus = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameExternalSecrets)
			return externalSecretsEntry
		})
	}

	// check for metallb
	wg.Add(1)
	go SysCheckExec("CheckMetalLb", &wg, &entries, func() SystemCheckEntry {
		metallbVersion, metallbInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, "metallb-controller")
		metallbMsg := fmt.Sprintf("%s (Version: %s) is installed.", NameMetalLB, metallbVersion)
		currentMetallbVersion := getMostCurrentHelmChartVersion(MetalLBHelmIndex, utils.HelmReleaseNameMetalLb)
		metallbEntry := CreateSystemCheckEntry(
			NameMetalLB,
			metallbInstalledErr == nil,
			metallbMsg,
			fmt.Sprintf("%s is not installed.\nTo have a local load balancer, you need to install this component.", NameMetalLB),
			metallbInstalledErr,
			"A load balancer for local clusters (e.g. Docker Desktop, k3s, minikube, etc.).",
			false,
			true,
			metallbVersion,
			currentMetallbVersion)
		metallbEntry.InstallPattern = structs.PAT_INSTALL_METALLB
		metallbEntry.UninstallPattern = structs.PAT_UNINSTALL_METALLB
		metallbEntry.UpgradePattern = "" // structs.PAT_UPGRADE_METALLB
		metallbEntry.HelmStatus = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameMetalLb)
		return metallbEntry
	})

	// TODO: FIXEN UND WIEDER EINBAUEN: MOG-1051
	// keplerVersion, keplerInstalledErr := punq.IsDaemonSetInstalled(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameKepler)
	// keplerMsg := fmt.Sprintf("%s (Version: %s) is installed.", NameKepler, keplerVersion)
	// if keplerInstalledErr != nil {
	// 	keplerMsg = fmt.Sprintf("%s is not installed.\nTo observe the power consumption of the cluster, you need to install this component.", NameKepler)
	// }
	// keplerDescription := "Kepler (Kubernetes-based Efficient Power Level Exporter) estimates workload energy/power consumption."
	// currentKeplerVersion := getMostCurrentHelmChartVersion(KeplerHelmIndex, utils.HelmReleaseNameKepler)
	// keplerEntry := CreateSystemCheckEntry(NameKepler, keplerInstalledErr == nil, keplerMsg, keplerDescription, false, false, keplerVersion, currentKeplerVersion)
	// keplerEntry.InstallPattern = structs.PAT_INSTALL_KEPLER
	// keplerEntry.UninstallPattern = structs.PAT_UNINSTALL_KEPLER
	// keplerEntry.UpgradePattern = "" // structs.PAT_UPGRADE_KEPLER
	// keplerEntry.Status = mokubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameKepler)
	// entries = append(entries, keplerEntry)

	// check for local dev setup
	wg.Add(1)
	go SysCheckExec("CheckLocalDevSetup", &wg, &entries, func() SystemCheckEntry {
		clusterIps := punq.GetClusterExternalIps(nil)
		localDevEnvMsg := "Local development environment setup complete (192.168.66.1 found)."
		contains192168661 := punqUtils.Contains(clusterIps, "192.168.66.1")
		if !contains192168661 {
			localDevEnvMsg = "Local development environment not setup. Please run 'mocli cluster local-dev-setup' to setup your local environment."
		}
		localDevSetupEntry := CreateSystemCheckEntry(
			NameLocalDevSetup,
			contains192168661,
			localDevEnvMsg,
			"",
			nil,
			"",
			false,
			false,
			"",
			"")
		return localDevSetupEntry
	})

	// check for nfs storage class
	wg.Add(1)
	go SysCheckExec("CheckNfsStorageClass", &wg, &entries, func() SystemCheckEntry {
		nfsStorageClass := kubernetes.StorageClassForClusterProvider(utils.ClusterProviderCached)
		nfsStorageClassMsg := fmt.Sprintf("NFS StorageClass '%s' found.", nfsStorageClass)
		var nfsStorageClassErr error
		if nfsStorageClass == "" {
			nfsStorageClassErr = fmt.Errorf("No default storage class found for cluster provider '%s'.", utils.ClusterProviderCached)
		}
		nfsStorageClassEntry := CreateSystemCheckEntry(
			NameNfsStorageClass,
			nfsStorageClass != "",
			nfsStorageClassMsg,
			"Please flag a default storage as default so it can be used for the nfs server.",
			nfsStorageClassErr,
			"",
			true,
			false,
			"",
			"")
		return nfsStorageClassEntry
	})

	wg.Wait()

	// update entries specificly for certain cluster vendors
	entries = UpdateSystemCheckStatusForClusterVendor(entries)

	// Sort the entries by CheckName
	sort.Sort(ByCheckName(entries))

	return GenerateSystemCheckResponse(entries)
}

func GenerateSystemCheckResponse(entries []SystemCheckEntry) SystemCheckResponse {
	return SystemCheckResponse{
		TerminalString: SystemCheckTerminalString(entries),
		Entries:        entries,
	}
}

func SystemCheckTerminalString(entries []SystemCheckEntry) string {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Check", "HelmStatus", "IsRunning", "Required", "ExecTime", "Message"})
	for index, entry := range entries {
		reqStr := "yes"
		if !entry.IsRequired {
			reqStr = "no"
		}
		isRunningStr := "yes"
		if !entry.IsRunning {
			isRunningStr = "no"
		}
		if entry.ErrorMessage != nil {
			t.AppendRow(
				table.Row{entry.CheckName, StatusEmoji(entry.HelmStatus), isRunningStr, reqStr, entry.ProcessTimeInMs, entry.ErrorMessage},
			)
		} else {
			t.AppendRow(
				table.Row{entry.CheckName, StatusEmoji(entry.HelmStatus), isRunningStr, reqStr, entry.ProcessTimeInMs, entry.SuccessMessage},
			)
		}
		if index < len(entries)-1 {
			t.AppendSeparator()
		}
	}

	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 2, Align: text.AlignCenter},
		{Number: 3, Align: text.AlignCenter},
	})

	return t.Render()
}

func StatusEmoji(state release.Status) string {
	switch state {
	case release.StatusPendingInstall, release.StatusUninstalled, release.StatusPendingUpgrade, release.StatusPendingRollback:
		return "🔧"
	case release.StatusFailed:
		return "❌"
	case release.StatusDeployed:
		return "✅"
	case release.StatusSuperseded:
		return "🔄"
	default:
		return "❓"
	}
}

func UpdateSystemCheckStatusForClusterVendor(entries []SystemCheckEntry) []SystemCheckEntry {
	provider, err := punq.GuessClusterProvider(nil)
	if err != nil {
		ServiceLogger.Errorf("UpdateSystemCheckStatusForClusterVendor Err: %s", err.Error())
		return entries
	}

	switch provider {
	case punqDtos.EKS, punqDtos.AKS, punqDtos.GKE, punqDtos.DOKS, punqDtos.OTC:
		entries = deleteSystemCheckEntryByName(entries, NameMetricsServer)
		entries = deleteSystemCheckEntryByName(entries, NameMetalLB)
		entries = deleteSystemCheckEntryByName(entries, NameLocalDevSetup)
	case punqDtos.UNKNOWN:
		ServiceLogger.Warnf("Unknown ClusterProvider. Not modifying anything in UpdateSystemCheckStatusForClusterVendor().")
	}

	// if public IP is available we skip metallLB
	nodes := kubernetes.ListNodes()
	for _, node := range nodes {
		for _, addr := range node.Status.Addresses {
			ip, err := netip.ParseAddr(addr.Address)
			if err == nil && !ip.IsPrivate() && ip.Is4() {
				entries = deleteSystemCheckEntryByName(entries, NameMetalLB)
			}
		}
	}
	lbIps := punq.GetClusterExternalIps(nil)
	for _, ip := range lbIps {
		ip, err := netip.ParseAddr(ip)
		if err == nil && !ip.IsPrivate() && ip.Is4() {
			entries = deleteSystemCheckEntryByName(entries, NameMetalLB)
		}
	}

	return entries
}

func deleteSystemCheckEntryByName(entries []SystemCheckEntry, name string) []SystemCheckEntry {
	for i := 0; i < len(entries); i++ {
		if entries[i].CheckName == name {
			entries = append(entries[:i], entries[i+1:]...)
			break
		}
	}
	return entries
}

func IsDockerInstalled() (bool, string, error) {
	cmd := punqUtils.RunOnLocalShell("/usr/local/bin/docker --version")
	output, err := cmd.CombinedOutput()
	return err == nil, strings.TrimRight(string(output), "\n\r"), err
}
