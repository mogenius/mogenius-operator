package services

import (
	"fmt"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"net/netip"
	"os"
	"strings"
	"sync"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	punqDtos "github.com/mogenius/punq/dtos"
	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
)

type SystemCheckEntry struct {
	CheckName          string                    `json:"checkName"`
	Status             structs.SystemCheckStatus `json:"status"`
	Message            string                    `json:"message"`
	Description        string                    `json:"description"`
	InstallPattern     string                    `json:"installPattern"`
	UpgradePattern     string                    `json:"upgradePattern"`
	UninstallPattern   string                    `json:"uninstallPattern"`
	IsRequired         bool                      `json:"isRequired"`
	WantsToBeInstalled bool                      `json:"wantsToBeInstalled"`
	VersionInstalled   string                    `json:"versionInstalled"`
	VersionAvailable   string                    `json:"versionAvailable"`
}

type SystemCheckResponse struct {
	TerminalString string             `json:"terminalString"`
	Entries        []SystemCheckEntry `json:"entries"`
}

func CreateSystemCheckEntry(checkName string, alreadyInstalled bool, message string, description string, isRequired bool, wantsToBeInstalled bool, versionInstalled string, versionAvailable string) SystemCheckEntry {
	status := structs.UNKNOWN_STATUS
	if alreadyInstalled {
		status = structs.INSTALLED
	} else {
		status = structs.NOT_INSTALLED
	}
	return SystemCheckEntry{
		CheckName:          checkName,
		Status:             status,
		Message:            message,
		Description:        description,
		IsRequired:         isRequired,
		WantsToBeInstalled: wantsToBeInstalled,
		VersionInstalled:   versionInstalled,
		VersionAvailable:   versionAvailable,
	}
}

func SystemCheck() SystemCheckResponse {
	var wg sync.WaitGroup
	entries := []SystemCheckEntry{}

	// check internet access
	wg.Add(1)
	go func() {
		defer wg.Done()
		inetResult, inetErr := punqUtils.CheckInternetAccess()
		inetMsg := StatusMessage(inetErr, "Check your internet connection.", "Internet access works.")
		entries = append(entries, CreateSystemCheckEntry("Internet Access", inetResult, inetMsg, "", true, false, "", ""))
	}()

	// check for kubectl + kubernetes version
	wg.Add(1)
	go func() {
		defer wg.Done()
		kubectlResult, kubectlOutput, kubectlErr := punqUtils.IsKubectlInstalled()
		kubeCtlMsg := StatusMessage(kubectlErr, "Plase install kubectl (https://kubernetes.io/docs/tasks/tools/) on your system to proceed.", kubectlOutput)
		entries = append(entries, CreateSystemCheckEntry("kubectl", kubectlResult, kubeCtlMsg, "", true, false, "", ""))

		kubernetesVersion := punq.KubernetesVersion(nil)
		kubernetesVersionResult := kubernetesVersion != nil
		kubernetesVersionMsg := StatusMessage(kubectlErr, "Cannot determin version of kubernetes.", fmt.Sprintf("Version: %s\nPlatform: %s", kubernetesVersion.String(), kubernetesVersion.Platform))
		entries = append(entries, CreateSystemCheckEntry("Kubernetes Version", kubernetesVersionResult, kubernetesVersionMsg, "", true, false, kubernetesVersion.String(), ""))
	}()

	// check for ingresscontroller
	wg.Add(1)
	go func() {
		defer wg.Done()
		ingressType, ingressTypeErr := punq.DetermineIngressControllerType(nil)
		ingressMsg := StatusMessage(ingressTypeErr, "Cannot determin ingress controller type.", ingressType.String())
		ingressDescription := "Installs a traefik ingress controller to handle traffic from outside the cluster and more."
		ingrEntry := CreateSystemCheckEntry("Ingress Controller", ingressTypeErr == nil, ingressMsg, ingressDescription, false, true, "", "")
		ingrEntry.InstallPattern = structs.PAT_INSTALL_INGRESS_CONTROLLER_TREAFIK
		ingrEntry.UninstallPattern = structs.PAT_UNINSTALL_INGRESS_CONTROLLER_TREAFIK
		ingrEntry.UpgradePattern = "" // structs.PAT_UPGRADE_INGRESS_CONTROLLER_TREAFIK
		ingrEntry.VersionAvailable = getMostCurrentHelmChartVersion(IngressControllerTraefikHelmIndex, utils.HelmReleaseNameTraefik)
		if ingrEntry.Status != structs.INSTALLED {
			ingrEntry.Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameTraefik)
		}
		entries = append(entries, ingrEntry)
	}()

	// check for metrics server
	wg.Add(1)
	go func() {
		defer wg.Done()
		metricsResult, metricsVersion, metricsErr := punq.IsMetricsServerAvailable(nil)
		metricsMsg := StatusMessage(metricsErr, "kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml\nNote: Running docker-desktop? Please add '- --kubelet-insecure-tls' to the args sction in the deployment of metrics-server.", metricsVersion)
		metricsDescription := "Maintained by Kubernetes-SIGs, handles metrics for built-in autoscaling pipelines."
		metricsEntry := CreateSystemCheckEntry("Metrics Server", metricsResult, metricsMsg, metricsDescription, true, true, metricsVersion, "")
		metricsEntry.InstallPattern = structs.PAT_INSTALL_METRICS_SERVER
		metricsEntry.UninstallPattern = structs.PAT_UNINSTALL_METRICS_SERVER
		metricsEntry.UpgradePattern = "" // structs.PAT_UPGRADE_METRICS_SERVER
		metricsEntry.VersionAvailable = getMostCurrentHelmChartVersion(MetricsHelmIndex, utils.HelmReleaseNameMetricsServer)
		if metricsEntry.Status != structs.INSTALLED {
			metricsEntry.Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameMetricsServer)
		}
		entries = append(entries)
	}()

	// check for helm
	wg.Add(1)
	go func() {
		defer wg.Done()
		helmResult, helmOutput, helmErr := punqUtils.IsHelmInstalled()
		helmMsg := StatusMessage(helmErr, "Plase install helm (https://helm.sh/docs/intro/install/) on your system to proceed.", helmOutput)
		entries = append(entries, CreateSystemCheckEntry("Helm", helmResult, helmMsg, "", true, false, helmOutput, ""))
	}()

	// check cluster provider
	wg.Add(1)
	go func() {
		defer wg.Done()
		clusterProvOutput, clusterProvErr := punq.GuessClusterProvider(nil)
		clusterProviderMsg := StatusMessage(clusterProvErr, "We could not determine the provider of this cluster.", string(clusterProvOutput))
		entries = append(entries, CreateSystemCheckEntry("Cluster Provider", clusterProvErr == nil, clusterProviderMsg, "", false, false, "", ""))
	}()

	// API Versions
	wg.Add(1)
	go func() {
		defer wg.Done()
		apiVerResult, apiVerErr := punq.ApiVersions(nil)
		apiVersStr := ""
		for _, entry := range apiVerResult {
			apiVersStr += fmt.Sprintf("%s\n", entry)
		}
		apiVersStr = strings.TrimRight(apiVersStr, "\n\r")
		apiVersMsg := StatusMessage(apiVerErr, "Metrics Server might be missing. Install the metrics server and try again.", apiVersStr)
		entries = append(entries, CreateSystemCheckEntry("API Versions", len(apiVerResult) > 0, apiVersMsg, "", true, false, "", ""))
	}()

	// check cluster provider
	wg.Add(1)
	go func() {
		defer wg.Done()
		countryResult, countryErr := punqUtils.GuessClusterCountry()
		countryName := ""
		if countryResult != nil {
			countryName = countryResult.Name
		}
		countryMsg := StatusMessage(countryErr, "We could not determine the location of the cluster.", countryName)
		entries = append(entries, CreateSystemCheckEntry("Cluster Country", countryErr == nil, countryMsg, "", false, false, "", ""))
	}()

	// check for loadbalancer ips
	wg.Add(1)
	go func() {
		defer wg.Done()
		lbName := "LoadBalancer IPs/Hosts"
		loadbalancerIps := punq.GetClusterExternalIps(nil)
		lbIpsMsg := strings.Join(loadbalancerIps, ", ")
		if len(loadbalancerIps) == 0 {
			lbIpsMsg = "No external IPs/Hosts.\nMaybe you don't have TREAFIK or NGINX ingress controller installed."
		}
		entries = append(entries, CreateSystemCheckEntry(lbName, len(loadbalancerIps) > 0, lbIpsMsg, "", false, false, "", ""))
	}()

	// check for docker
	wg.Add(1)
	go func() {
		defer wg.Done()
		dockerResult, dockerOutput, dockerErr := IsDockerInstalled()
		dockerMsg := StatusMessage(dockerErr, "If docker is missing in this image, we are screwed ;-)", dockerOutput)
		entries = append(entries, CreateSystemCheckEntry("docker", dockerResult, dockerMsg, "", true, false, dockerOutput, ""))
	}()

	// check for cert-manager
	wg.Add(1)
	go func() {
		defer wg.Done()
		certManagerVersion, certManagerInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameCertManager)
		certManagerMsg := fmt.Sprintf("%s (Version: %s) is installed.", utils.HelmReleaseNameCertManager, certManagerVersion)
		if certManagerInstalledErr != nil {
			certManagerMsg = fmt.Sprintf("%s is not installed.\nTo create ssl certificates you need to install this component.", utils.HelmReleaseNameCertManager)
		}
		certMgrDescription := "Install the cert-manager to automatically issue Let's Encrypt certificates to your services."
		currentCertManagerVersion := getMostCurrentHelmChartVersion(CertManagerHelmIndex, utils.HelmReleaseNameCertManager)
		certMgrEntry := CreateSystemCheckEntry(utils.HelmReleaseNameCertManager, certManagerInstalledErr == nil, certManagerMsg, certMgrDescription, false, true, certManagerVersion, currentCertManagerVersion)
		certMgrEntry.InstallPattern = structs.PAT_INSTALL_CERT_MANAGER
		certMgrEntry.UninstallPattern = structs.PAT_UNINSTALL_CERT_MANAGER
		certMgrEntry.UpgradePattern = "" // structs.PAT_UPGRADE_CERT_MANAGER
		certMgrEntry.Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameCertManager)
		entries = append(entries, certMgrEntry)
	}()

	// check for clusterissuer
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, clusterIssuerInstalledErr := punq.GetClusterIssuer(NameClusterIssuerResource, nil)
		clusterIssuerMsg := fmt.Sprintf("%s is installed.", NameClusterIssuerResource)
		if clusterIssuerInstalledErr != nil {
			clusterIssuerMsg = fmt.Sprintf("%s is not installed.\nTo issue ssl certificates you need to install this component.", NameClusterIssuerResource)
		}
		clusterIssuerDescription := "Responsible for signing certificates."
		clusterIssuerEntry := CreateSystemCheckEntry(utils.HelmReleaseNameClusterIssuer, clusterIssuerInstalledErr == nil, clusterIssuerMsg, clusterIssuerDescription, false, true, "", "")
		clusterIssuerEntry.InstallPattern = structs.PAT_INSTALL_CLUSTER_ISSUER
		clusterIssuerEntry.UninstallPattern = structs.PAT_UNINSTALL_CLUSTER_ISSUER
		clusterIssuerEntry.Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameClusterIssuer)
		entries = append(entries, clusterIssuerEntry)
	}()

	// check internal tools version for later usage
	trafficCollectorNewestVersion, podstatsCollectorNewestVersion, err := getCurrentTrafficCollectorAndPodStatsVersion()
	if err != nil {
		log.Errorf("getCurrentTrafficCollectorVersion Err: %s", err.Error())
	}

	// check for trafficcollector
	wg.Add(1)
	go func() {
		defer wg.Done()
		trafficCollectorVersion, trafficCollectorInstalledErr := punq.IsDaemonSetInstalled(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameTrafficCollector)
		if trafficCollectorVersion == "" && trafficCollectorInstalledErr == nil {
			trafficCollectorVersion = "6.6.6" // flag local version without tag
		}
		trafficMsg := fmt.Sprintf("%s (Version: %s) is installed.", utils.HelmReleaseNameTrafficCollector, trafficCollectorVersion)
		if trafficCollectorInstalledErr != nil {
			trafficMsg = fmt.Sprintf("%s is not installed.\nTo gather traffic information you need to install this component.", utils.HelmReleaseNameTrafficCollector)
		}
		trafficDescpription := fmt.Sprintf("Collects and exposes detailed traffic data for your mogenius services for better monitoring. (Installed: %s | Available: %s)", trafficCollectorVersion, trafficCollectorNewestVersion)
		trafficEntry := CreateSystemCheckEntry(utils.HelmReleaseNameTrafficCollector, trafficCollectorInstalledErr == nil, trafficMsg, trafficDescpription, false, true, trafficCollectorVersion, trafficCollectorNewestVersion)
		trafficEntry.InstallPattern = structs.PAT_INSTALL_TRAFFIC_COLLECTOR
		trafficEntry.UninstallPattern = structs.PAT_UNINSTALL_TRAFFIC_COLLECTOR
		trafficEntry.UpgradePattern = structs.PAT_UPGRADE_TRAFFIC_COLLECTOR
		trafficEntry.Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameTrafficCollector)
		entries = append(entries, trafficEntry)
	}()

	// check for podstatscollector
	wg.Add(1)
	go func() {
		defer wg.Done()
		podStatsCollectorVersion, podStatsCollectorInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNamePodStatsCollector)
		if podStatsCollectorVersion == "" && podStatsCollectorInstalledErr == nil {
			podStatsCollectorVersion = "6.6.6" // flag local version without tag
		}
		podStatsMsg := fmt.Sprintf("%s (Version: %s) is installed.", utils.HelmReleaseNamePodStatsCollector, podStatsCollectorVersion)
		if podStatsCollectorInstalledErr != nil {
			podStatsMsg = fmt.Sprintf("%s is not installed.\nTo gather pod/event information you need to install this component.", utils.HelmReleaseNamePodStatsCollector)
		}
		podStatsDescription := fmt.Sprintf("Collects and exposes status events of pods for services in mogenius. (Installed: %s | Available: %s)", podStatsCollectorVersion, podstatsCollectorNewestVersion)
		podEntry := CreateSystemCheckEntry(utils.HelmReleaseNamePodStatsCollector, podStatsCollectorInstalledErr == nil, podStatsMsg, podStatsDescription, true, true, podStatsCollectorVersion, podstatsCollectorNewestVersion)
		podEntry.InstallPattern = structs.PAT_INSTALL_POD_STATS_COLLECTOR
		podEntry.UninstallPattern = structs.PAT_UNINSTALL_POD_STATS_COLLECTOR
		podEntry.UpgradePattern = structs.PAT_UPGRADE_PODSTATS_COLLECTOR
		podEntry.Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNamePodStatsCollector)
		entries = append(entries, podEntry)
	}()

	// check for distribution registry
	wg.Add(1)
	go func() {
		defer wg.Done()
		distributionRegistryName := "distribution-registry-docker-registry"
		distriRegistryVersion, distriRegistryInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, distributionRegistryName)
		distriRegistryMsg := fmt.Sprintf("%s (Version: %s) is installed.", distributionRegistryName, distriRegistryVersion)
		if distriRegistryInstalledErr != nil {
			distriRegistryMsg = fmt.Sprintf("%s is not installed.\nTo have a private container registry running inside your cluster, you need to install this component.", distributionRegistryName)
		}
		distriDescription := "A Docker-based container registry inside Kubernetes."
		currentDistriRegistryVersion := getMostCurrentHelmChartVersion(ContainerRegistryHelmIndex, "docker-registry")
		distriEntry := CreateSystemCheckEntry(NameInternalContainerRegistry, distriRegistryInstalledErr == nil, distriRegistryMsg, distriDescription, false, true, distriRegistryVersion, currentDistriRegistryVersion)
		distriEntry.InstallPattern = structs.PAT_INSTALL_CONTAINER_REGISTRY
		distriEntry.UninstallPattern = structs.PAT_UNINSTALL_CONTAINER_REGISTRY
		distriEntry.UpgradePattern = "" // structs.PAT_UPGRADE_CONTAINER_REGISTRY
		distriEntry.Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameDistributionRegistry)
		entries = append(entries, distriEntry)
	}()

	// externalSecretsName := "external-secrets"
	// externalSecretsVersion, externalSecretsInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, externalSecretsName)
	// externalSecretsMsg := fmt.Sprintf("%s (Version: %s) is installed.", externalSecretsName, externalSecretsVersion)
	// if externalSecretsInstalledErr != nil {
	// 	externalSecretsMsg = fmt.Sprintf("%s is not installed.\nTo load secrets from 3rd party vaults (e.g. e.g. Hashicorp Vault, AWS KMS or Azure Key Vault), you need to install this component.", externalSecretsName)
	// }
	// externalSecretsDescription := "A Docker-based External Secrets loader inside Kubernetes that allows you to connect to e.g. Hashicorp Vault, AWS KMS or Azure Key Vault"
	// currentExternalSecretsVersion := getMostCurrentHelmChartVersion(ExternalSecretsHelmIndex, "docker-registry")
	// externalSecretsEntry := CreateSystemCheckEntry(NameExternalSecrets, externalSecretsInstalledErr == nil, externalSecretsMsg, externalSecretsDescription, false, true, externalSecretsVersion, currentExternalSecretsVersion)
	// externalSecretsEntry.InstallPattern = structs.PAT_INSTALL_EXTERNAL_SECRETS
	// externalSecretsEntry.UninstallPattern = structs.PAT_UNINSTALL_EXTERNAL_SECRETS
	// externalSecretsEntry.UpgradePattern = "" // NONE?
	// externalSecretsEntry.Status = mokubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameExternalSecrets)
	// entries = append(entries, externalSecretsEntry)

	// check for metallb
	wg.Add(1)
	go func() {
		defer wg.Done()
		metallbVersion, metallbInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, "metallb-controller")
		metallbMsg := fmt.Sprintf("%s (Version: %s) is installed.", NameMetalLB, metallbVersion)
		if metallbInstalledErr != nil {
			metallbMsg = fmt.Sprintf("%s is not installed.\nTo have a local load balancer, you need to install this component.", NameMetalLB)
		}
		metallbDescription := "A load balancer for local clusters (e.g. Docker Desktop, k3s, minikube, etc.)."
		currentMetallbVersion := getMostCurrentHelmChartVersion(MetalLBHelmIndex, utils.HelmReleaseNameMetalLb)
		metallbEntry := CreateSystemCheckEntry(NameMetalLB, metallbInstalledErr == nil, metallbMsg, metallbDescription, false, true, metallbVersion, currentMetallbVersion)
		metallbEntry.InstallPattern = structs.PAT_INSTALL_METALLB
		metallbEntry.UninstallPattern = structs.PAT_UNINSTALL_METALLB
		metallbEntry.UpgradePattern = "" // structs.PAT_UPGRADE_METALLB
		metallbEntry.Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameMetalLb)
		entries = append(entries, metallbEntry)
	}()

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
	go func() {
		defer wg.Done()
		clusterIps := punq.GetClusterExternalIps(nil)
		localDevEnvMsg := "Local development environment setup complete (192.168.66.1 found)."
		contains192168661 := punqUtils.Contains(clusterIps, "192.168.66.1")
		if !contains192168661 {
			localDevEnvMsg = "Local development environment not setup. Please run 'mocli cluster local-dev-setup' to setup your local environment."
		}
		localDevSetupEntry := CreateSystemCheckEntry(NameLocalDevSetup, contains192168661, localDevEnvMsg, "", false, false, "", "")
		entries = append(entries, localDevSetupEntry)

		nfsStorageClass := kubernetes.StorageClassForClusterProvider(utils.ClusterProviderCached)
		nfsStorageClassMsg := fmt.Sprintf("NFS StorageClass '%s' found.", nfsStorageClass)
		nfsStorageClassEntry := CreateSystemCheckEntry(NameNfsStorageClass, nfsStorageClass != "", nfsStorageClassMsg, "", true, false, "", "")
		entries = append(entries, nfsStorageClassEntry)
	}()

	wg.Wait()

	// update entries specificly for certain cluster vendors
	entries = UpdateSystemCheckStatusForClusterVendor(entries)

	return GenerateSystemCheckResponse(entries)
}

func StatusMessage(err error, solution string, successMsg string) string {
	if err != nil {
		return fmt.Sprintf("Error: %s\nSolution: %s", err.Error(), solution)
	}
	return successMsg
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
	t.AppendHeader(table.Row{"Check", "Status", "Required", "Message"})
	for index, entry := range entries {
		reqStr := "yes"
		if !entry.IsRequired {
			reqStr = "no"
		}
		t.AppendRow(
			table.Row{entry.CheckName, StatusEmoji(entry.Status), reqStr, entry.Message},
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

func StatusEmoji(state structs.SystemCheckStatus) string {
	switch state {
	case structs.UNKNOWN_STATUS:
		return "❓"
	case structs.INSTALLING:
		return "🔧"
	case structs.UNINSTALLING:
		return "🔧"
	case structs.NOT_INSTALLED:
		return "❌"
	case structs.INSTALLED:
		return "✅"
	default:
		return "❓"
	}
}

func UpdateSystemCheckStatusForClusterVendor(entries []SystemCheckEntry) []SystemCheckEntry {
	provider, err := punq.GuessClusterProvider(nil)
	if err != nil {
		log.Errorf("UpdateSystemCheckStatusForClusterVendor Err: %s", err.Error())
		return entries
	}

	switch provider {
	case punqDtos.EKS, punqDtos.AKS, punqDtos.GKE, punqDtos.DOKS, punqDtos.OTC:
		entries = deleteSystemCheckEntryByName(entries, NameMetricsServer)
		entries = deleteSystemCheckEntryByName(entries, NameMetalLB)
		entries = deleteSystemCheckEntryByName(entries, NameLocalDevSetup)
	case punqDtos.UNKNOWN:
		log.Warnf("Unknown ClusterProvider. Not modifying anything in UpdateSystemCheckStatusForClusterVendor().")
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
