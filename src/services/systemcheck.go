package services

import (
	"fmt"
	"mogenius-k8s-manager/src/helm"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"helm.sh/helm/v3/pkg/release"
)

type SystemCheckEntry struct {
	CheckName          string         `json:"checkName"`
	HelmStatus         release.Status `json:"helmStatus"`
	IsRunning          bool           `json:"isRunning"`
	SuccessMessage     string         `json:"successMessage"`
	ErrorMessage       *string        `json:"errorMessage,omitempty"`
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

// sysCheckExec measures the execution time of a function
func sysCheckExec(name string, entries *[]SystemCheckEntry, fn func() SystemCheckEntry) {
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
	wg.Go(func() {
		sysCheckExec("CheckInternetAccess", &entries, func() SystemCheckEntry {
			inetResult, inetErr := utils.CheckInternetAccess()
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
	})

	// check kubernetes version
	wg.Go(func() {
		sysCheckExec("CheckKubernetesVersion", &entries, func() SystemCheckEntry {
			kubernetesVersion := kubernetes.KubernetesVersion()
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
	})

	// check for ingresscontroller
	wg.Go(func() {
		sysCheckExec("CheckIngressController", &entries, func() SystemCheckEntry {
			ingressType, ingressTypeErr := kubernetes.DetermineIngressControllerType()
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
			ingrEntry.HelmStatus = helm.HelmStatus(config.Get("MO_OWN_NAMESPACE"), utils.HelmReleaseNameTraefik)
			return ingrEntry
		})
	})

	// check for metrics server
	wg.Go(func() {
		sysCheckExec("CheckMetricsServer", &entries, func() SystemCheckEntry {
			metricsResult, metricsVersion, metricsErr := kubernetes.IsMetricsServerAvailable()
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
			metricsEntry.HelmStatus = helm.HelmStatus(config.Get("MO_OWN_NAMESPACE"), utils.HelmReleaseNameMetricsServer)
			return metricsEntry
		})
	})

	// check cluster provider
	wg.Go(func() {
		sysCheckExec("CheckClusterProvider", &entries, func() SystemCheckEntry {
			clusterProvOutput, clusterProvErr := kubernetes.GuessClusterProvider()
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
	})

	// API Versions
	wg.Go(func() {
		sysCheckExec("CheckApiVersions", &entries, func() SystemCheckEntry {
			apiVerResult, apiVerErr := kubernetes.ApiVersions()
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
	})

	// check country of cluster
	wg.Go(func() {
		sysCheckExec("CheckClusterCountry", &entries, func() SystemCheckEntry {
			countryResult, countryErr := utils.GuessClusterCountry()
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
	})

	// check for loadbalancer ips
	wg.Go(func() {
		sysCheckExec("CheckLoadBalancerIps", &entries, func() SystemCheckEntry {
			lbName := "LoadBalancer IPs/Hosts"
			loadbalancerIps := kubernetes.GetClusterExternalIps()
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
	})

	// check for cert-manager
	wg.Go(func() {
		sysCheckExec("CheckCertManager", &entries, func() SystemCheckEntry {
			certMangerResultArray, certManagerInstalledErr := kubernetes.GetDeploymentsWithFieldSelector("", "app=cert-manager")
			certManagerVersion := "not installed"
			if len(certMangerResultArray) > 0 {
				certManagerVersion = certMangerResultArray[0].Labels["app.kubernetes.io/version"]
			} else {
				certManagerInstalledErr = fmt.Errorf("Cert-Manager not installed.")
			}
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
			certMgrEntry.HelmStatus = helm.HelmStatus(config.Get("MO_OWN_NAMESPACE"), utils.HelmReleaseNameCertManager)
			return certMgrEntry
		})
	})

	// check for clusterissuer
	wg.Go(func() {
		sysCheckExec("CheckClusterIssuer", &entries, func() SystemCheckEntry {
			_, clusterIssuerInstalledErr := kubernetes.GetClusterIssuer(NameClusterIssuerResource)
			clusterIssuerMsg := fmt.Sprintf("%s is installed.", NameClusterIssuerResource)
			helmstatus := helm.HelmStatus(config.Get("MO_OWN_NAMESPACE"), utils.HelmReleaseNameClusterIssuer)
			if helmstatus == release.StatusUnknown {
				clusterIssuerInstalledErr = nil
				clusterIssuerMsg = "Cluster Issuer not installed."
			}
			clusterIssuerEntry := CreateSystemCheckEntry(
				utils.HelmReleaseNameClusterIssuer,
				clusterIssuerInstalledErr == nil && helmstatus != release.StatusUnknown,
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
			clusterIssuerEntry.HelmStatus = helmstatus
			return clusterIssuerEntry
		})
	})

	// check for metallb
	wg.Go(func() {
		sysCheckExec("CheckMetalLb", &entries, func() SystemCheckEntry {
			metallbVersion, metallbInstalledErr := kubernetes.IsDeploymentInstalled(config.Get("MO_OWN_NAMESPACE"), "metallb-controller")
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
			metallbEntry.HelmStatus = helm.HelmStatus(config.Get("MO_OWN_NAMESPACE"), utils.HelmReleaseNameMetalLb)
			return metallbEntry
		})
	})

	// check for nfs storage class
	wg.Go(func() {
		sysCheckExec("CheckNfsStorageClass", &entries, func() SystemCheckEntry {
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
	t.AppendHeader(table.Row{"Check", "HelmStatus", "IsRunning", "Required", "ExecTime", "Message", "Error"})
	for index, entry := range entries {
		reqStr := "yes"
		if !entry.IsRequired {
			reqStr = "no"
		}
		isRunningStr := "yes"
		if !entry.IsRunning {
			isRunningStr = "no"
		}

		errStr := ""
		if entry.ErrorMessage != nil {
			errStr = *entry.ErrorMessage
		}

		t.AppendRow(
			table.Row{entry.CheckName, StatusEmoji(entry.HelmStatus), isRunningStr, reqStr, entry.ProcessTimeInMs, entry.SuccessMessage, errStr},
		)
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
	provider, err := kubernetes.GuessClusterProvider()
	if err != nil {
		serviceLogger.Error("UpdateSystemCheckStatusForClusterVendor", "error", err)
		return entries
	}

	switch provider {
	case utils.EKS, utils.AKS, utils.GKE, utils.DOKS, utils.OTC, utils.PLUSSERVER:
		entries = deleteSystemCheckEntryByName(entries, NameMetalLB)
	case utils.UNKNOWN:
		serviceLogger.Warn("Unknown ClusterProvider. Not modifying anything in UpdateSystemCheckStatusForClusterVendor().")
	}

	// if public IP is available we skip metallLB
	nodes := store.GetNodes()
	for _, node := range nodes {
		for _, addr := range node.Status.Addresses {
			ip, err := netip.ParseAddr(addr.Address)
			if err == nil && !ip.IsPrivate() && ip.Is4() {
				entries = deleteSystemCheckEntryByName(entries, NameMetalLB)
			}
		}
	}
	lbIps := kubernetes.GetClusterExternalIps()
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
