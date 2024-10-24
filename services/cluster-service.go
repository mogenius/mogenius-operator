package services

import (
	"fmt"
	"io"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	punqDtos "github.com/mogenius/punq/dtos"
	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"

	"github.com/shirou/gopsutil/v3/disk"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	NameMetricsServer             = "Metrics Server"
	NameMetalLB                   = "MetalLB (LoadBalancer)"
	NameIngressController         = "Ingress Controller"
	NameLocalDevSetup             = "Local Dev Setup"
	NameInternalContainerRegistry = "Internal Container Registry"
	NameExternalSecrets           = "External Secrets"
	NameClusterIssuerResource     = "letsencrypt-cluster-issuer"
	NameKepler                    = "Kepler"
	NameNfsStorageClass           = "nfs-storageclass"
)

const (
	MetricsHelmIndex                  = "https://kubernetes-sigs.github.io/metrics-server"
	IngressControllerTraefikHelmIndex = "https://traefik.github.io/charts"
	ContainerRegistryHelmIndex        = "https://phntom.kix.co.il/charts"
	ExternalSecretsHelmIndex          = "https://charts.external-secrets.io"
	CertManagerHelmIndex              = "https://charts.jetstack.io"
	KeplerHelmIndex                   = "https://sustainable-computing-io.github.io/kepler-helm-chart"
	MetalLBHelmIndex                  = "https://metallb.github.io/metallb"
	MogeniusHelmIndex                 = "https://helm.mogenius.com/public"
)

func UpgradeK8sManager(r K8sManagerUpgradeRequest) *structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Upgrade mogenius platform", "UPGRADE", "", "")
	job.Start()
	mokubernetes.UpgradeMyself(job, r.Command, &wg)
	wg.Wait()
	job.Finish()
	return job
}

func InstallHelmChart(r ClusterHelmRequest) *structs.Job {
	job := structs.CreateJob("Install Helm Chart "+r.HelmReleaseName, r.NamespaceId, "", "")
	job.Start()
	result, err := mokubernetes.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
	if err != nil {
		job.Fail(fmt.Sprintf("Failed to install helm chart %s: %s\n%s", r.HelmReleaseName, result, err.Error()))
	}
	job.Finish()
	return job
}

func DeleteHelmChart(r ClusterHelmUninstallRequest) *structs.Job {
	job := structs.CreateJob("Delete Helm Chart "+r.HelmReleaseName, r.NamespaceId, "", "")
	job.Start()
	result, err := mokubernetes.DeleteHelmChart(r.HelmReleaseName, r.NamespaceId)
	if err != nil {
		job.Fail(fmt.Sprintf("Failed to delete helm chart %s: %s\n%s", r.HelmReleaseName, result, err.Error()))
	}
	job.Finish()
	return job
}

func CreateMogeniusNfsVolume(r NfsVolumeRequest) structs.DefaultResponse {
	var wg sync.WaitGroup
	job := structs.CreateJob("Create mogenius nfs-volume.", r.NamespaceName, "", "")
	job.Start()
	// FOR K8SMANAGER
	mokubernetes.CreateMogeniusNfsServiceSync(job, r.NamespaceName, r.VolumeName)
	mokubernetes.CreateMogeniusNfsPersistentVolumeClaim(job, r.NamespaceName, r.VolumeName, r.SizeInGb, &wg)
	mokubernetes.CreateMogeniusNfsDeployment(job, r.NamespaceName, r.VolumeName, &wg)
	// FOR SERVICES THAT WANT TO MOUNT
	mokubernetes.CreateMogeniusNfsPersistentVolumeForService(job, r.NamespaceName, r.VolumeName, r.SizeInGb, &wg)
	mokubernetes.CreateMogeniusNfsPersistentVolumeClaimForService(job, r.NamespaceName, r.VolumeName, r.SizeInGb, &wg)
	wg.Wait()
	job.Finish()

	nfsService := mokubernetes.ServiceForNfsVolume(r.NamespaceName, r.VolumeName)
	mokubernetes.Mount(r.NamespaceName, r.VolumeName, nfsService)

	return job.DefaultReponse()
}

func DeleteMogeniusNfsVolume(r NfsVolumeRequest) structs.DefaultResponse {
	var wg sync.WaitGroup
	job := structs.CreateJob("Delete mogenius nfs-volume.", r.NamespaceName, "", "")
	job.Start()
	// FOR K8SMANAGER
	mokubernetes.DeleteMogeniusNfsDeployment(job, r.NamespaceName, r.VolumeName, &wg)
	mokubernetes.DeleteMogeniusNfsService(job, r.NamespaceName, r.VolumeName, &wg)
	mokubernetes.DeleteMogeniusNfsPersistentVolumeClaim(job, r.NamespaceName, r.VolumeName, &wg)
	// FOR SERVICES THAT WANT TO MOUNT
	mokubernetes.DeleteMogeniusNfsPersistentVolumeForService(job, r.VolumeName, r.NamespaceName, &wg)
	mokubernetes.DeleteMogeniusNfsPersistentVolumeClaimForService(job, r.NamespaceName, r.VolumeName, &wg)
	wg.Wait()
	job.Finish()

	mokubernetes.Umount(r.NamespaceName, r.VolumeName)

	return job.DefaultReponse()
}

func StatsMogeniusNfsVolume(r NfsVolumeStatsRequest) NfsVolumeStatsResponse {
	mountPath := utils.MountPath(r.NamespaceName, r.VolumeName, "/")
	free, used, total, _ := diskUsage(mountPath)
	result := NfsVolumeStatsResponse{
		VolumeName: r.VolumeName,
		FreeBytes:  free,
		UsedBytes:  used,
		TotalBytes: total,
	}

	ServiceLogger.Infof("💾: '%s' -> %s / %s (Free: %s)", mountPath, punqUtils.BytesToHumanReadable(int64(result.UsedBytes)), punqUtils.BytesToHumanReadable(int64(result.TotalBytes)), punqUtils.BytesToHumanReadable(int64(result.FreeBytes)))
	return result
}

func diskUsage(mountPath string) (uint64, uint64, uint64, error) {
	usage, err := disk.Usage(mountPath)
	if err != nil {
		ServiceLogger.Errorf("StatsMogeniusNfsVolume Err: %s %s", mountPath, err.Error())
		return 0, 0, 0, err
	} else {
		return usage.Free, usage.Used, usage.Total, nil
	}
}

func StatsMogeniusNfsNamespace(r NfsNamespaceStatsRequest) []NfsVolumeStatsResponse {
	result := []NfsVolumeStatsResponse{}

	if r.NamespaceName == "null" || r.NamespaceName == "" {
		ServiceLogger.Errorf("StatsMogeniusNfsNamespace Err: namespaceName cannot be null or empty.")
		return result
	}

	// get all pvc for single namespace
	pvcs := punq.AllPersistentVolumeClaims(r.NamespaceName, nil)

	for _, pvc := range pvcs {
		// skip pvcs which are not mogenius-nfs
		if !strings.HasPrefix(pvc.Name, fmt.Sprintf("%s-", utils.CONFIG.Misc.NfsPodPrefix)) {
			continue
		}
		// remove podname "nfs-server-pod-"
		pvc.Name = strings.Replace(pvc.Name, fmt.Sprintf("%s-", utils.CONFIG.Misc.NfsPodPrefix), "", 1)

		entry := NfsVolumeStatsResponse{
			VolumeName: pvc.Name,
			FreeBytes:  0,
			UsedBytes:  0,
			TotalBytes: 0,
		}

		mountPath := utils.MountPath(r.NamespaceName, pvc.Name, "/")

		if utils.ClusterProviderCached == punqDtos.DOCKER_DESKTOP || utils.ClusterProviderCached == punqDtos.K3S {
			var usedBytes uint64 = sumAllBytesOfFolder(mountPath)
			entry.FreeBytes = uint64(pvc.Spec.Resources.Requests.Storage().Value()) - usedBytes
			entry.UsedBytes = usedBytes
			entry.TotalBytes = uint64(pvc.Spec.Resources.Requests.Storage().Value())
		} else {
			free, used, total, err := diskUsage(mountPath)
			if err != nil {
				continue
			} else {
				entry.FreeBytes = free
				entry.UsedBytes = used
				entry.TotalBytes = total
			}
		}

		ServiceLogger.Infof("💾: '%s' -> %s / %s (Free: %s)", mountPath, punqUtils.BytesToHumanReadable(int64(entry.UsedBytes)), punqUtils.BytesToHumanReadable(int64(entry.TotalBytes)), punqUtils.BytesToHumanReadable(int64(entry.FreeBytes)))
		result = append(result, entry)
	}
	return result
}

func sumAllBytesOfFolder(root string) uint64 {
	var total uint64
	var wg sync.WaitGroup
	var sumWg sync.WaitGroup
	fileSizes := make(chan uint64)

	sumWg.Add(1)
	// Start a goroutine to sum file sizes.
	go func() {
		defer sumWg.Done() // Signal completion of summing
		for size := range fileSizes {
			total += size
		}
	}()

	// Walk the file tree concurrently.
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			wg.Add(1)
			go func() {
				defer wg.Done()
				info, err := d.Info()
				if err != nil {
					return // handle error
				}
				fileSizes <- uint64(info.Size())
			}()
		}
		return nil
	})
	if err != nil {
		ServiceLogger.Errorf("Error while summing bytes in path: %s", err.Error())
	}

	wg.Wait()
	close(fileSizes) // Close channel to finish summing
	sumWg.Wait()     // Wait for summing to complete

	return total
}

// func UnzipAndReplaceFromS3(namespaceName string, volumeName string, BackupKey string, result NfsVolumeRestoreResponse, accessKeyId string, secretAccessKey string, token string) NfsVolumeRestoreResponse {
// 	// Set up an AWS session
// 	sess := session.Must(session.NewSession(&aws.Config{
// 		Region:      aws.String("eu-central-1"),
// 		Credentials: credentials.NewStaticCredentials(accessKeyId, secretAccessKey, token),
// 	}))

// 	// Download the zip file from S3
// 	downloader := s3manager.NewDownloader(sess)
// 	buffer := &aws.WriteAtBuffer{}
// 	downloadedBytes, err := downloader.Download(buffer, &s3.GetObjectInput{
// 		Bucket: aws.String(BUCKETNAME),
// 		Key:    aws.String(BackupKey),
// 	})
// 	if err != nil {
// 		ServiceLogger.Errorf("s3 Download error: %s", err.Error())
// 		result.Error = err.Error()
// 		return result
// 	}

// 	// Replace files with downloaded data
// 	r, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(len(buffer.Bytes())))
// 	if err != nil {
// 		panic(err)
// 	}

// 	mountPath := utils.MountPath(namespaceName, volumeName, "")
// 	mountPath = fmt.Sprintf("%s/restore", mountPath)
// 	err = os.MkdirAll(mountPath, 0755)
// 	if err != nil {
// 		ServiceLogger.Fatal(err)
// 	}

// 	for _, f := range r.File {
// 		rc, err := f.Open()
// 		if err != nil {
// 			ServiceLogger.Error(err)
// 		}
// 		defer rc.Close()

// 		// Create the destination file
// 		destFilepath := fmt.Sprintf("%s/%s", mountPath, f.Name)
// 		destFile, err := os.Create(destFilepath)
// 		if err != nil {
// 			ServiceLogger.Error(err)
// 		}
// 		defer destFile.Close()

// 		// Copy the contents of the source file to the destination file
// 		_, err = io.Copy(destFile, rc)
// 		if err != nil {
// 			ServiceLogger.Error(err)
// 		}

// 		// Print the name of the unzipped file
// 		if utils.CONFIG.Misc.Debug {
// 			ServiceLogger.Infof("Unzipped file: %s\n", destFilepath)
// 		}
// 	}

// 	msg := fmt.Sprintf("Successfully restored volume (%s) from S3!\n", punqUtils.BytesToHumanReadable(downloadedBytes))
// 	ServiceLogger.Info(msg)
// 	result.Message = msg

// 	return result
// }

// func ZipDirAndUploadToS3(directoryToZip string, targetFileName string, result NfsVolumeBackupResponse, accessKeyId string, secretAccessKey string, token string) NfsVolumeBackupResponse {
// 	// Set up an AWS session
// 	sess := session.Must(session.NewSession(&aws.Config{
// 		Region:      aws.String("eu-central-1"),
// 		Credentials: credentials.NewStaticCredentials(accessKeyId, secretAccessKey, token),
// 	}))

// 	// Create a zip archive buffer
// 	buf := new(bytes.Buffer)
// 	zipWriter := zip.NewWriter(buf)

// 	// Add all files in a directory to the archive
// 	err := filepath.Walk(directoryToZip, func(path string, info os.FileInfo, err error) error {
// 		if err != nil {
// 			return err
// 		}
// 		if info.IsDir() {
// 			return nil
// 		}

// 		fileBytes, err := os.ReadFile(path)
// 		if err != nil {
// 			return err
// 		}

// 		relPath, err := filepath.Rel(directoryToZip, path)
// 		if err != nil {
// 			return err
// 		}

// 		zipFile, err := zipWriter.Create(relPath)
// 		if err != nil {
// 			return err
// 		}

// 		_, err = io.Copy(zipFile, bytes.NewReader(fileBytes))
// 		if err != nil {
// 			return err
// 		}

// 		return nil
// 	})
// 	if err != nil {
// 		ServiceLogger.Errorf("s3 walk files error: %s", err.Error())
// 		result.Error = err.Error()
// 		return result
// 	}

// 	// Close the zip archive
// 	err = zipWriter.Close()
// 	if err != nil {
// 		ServiceLogger.Errorf("s3 zip error: %s", err.Error())
// 		result.Error = err.Error()
// 		return result
// 	}

// 	// Upload the zip file to S3
// 	s3svc := s3.New(sess)
// 	_, err = s3svc.PutObject(&s3.PutObjectInput{
// 		Bucket: aws.String(BUCKETNAME),     // Replace with your S3 bucket name
// 		Key:    aws.String(targetFileName), // Replace with the name you want to give the zip file in S3
// 		Body:   bytes.NewReader(buf.Bytes()),
// 	})
// 	if err != nil {
// 		ServiceLogger.Errorf("s3 Send error: %s", err.Error())
// 		result.Error = err.Error()
// 		return result
// 	}

// 	// Get the uploaded object and presign it.
// 	req, _ := s3svc.GetObjectRequest(&s3.GetObjectInput{
// 		Bucket: aws.String(BUCKETNAME),
// 		Key:    aws.String(targetFileName),
// 	})
// 	url, err := req.Presign(15 * time.Minute)
// 	if err != nil {
// 		ServiceLogger.Errorf("s3 presign error: %s", err.Error())
// 		result.Error = err.Error()
// 		return result
// 	}
// 	headObj, err := s3svc.HeadObject(&s3.HeadObjectInput{
// 		Bucket: aws.String(BUCKETNAME),
// 		Key:    aws.String(targetFileName),
// 	})
// 	if err != nil {
// 		ServiceLogger.Errorf("s3 headobject error: %s", err.Error())
// 		result.Error = err.Error()
// 		return result
// 	}

// 	result.DownloadUrl = url
// 	if headObj != nil {
// 		result.Bytes = *headObj.ContentLength
// 	}

// 	ServiceLogger.Infof("Successfully uploaded zip file (%s) to S3! -> %s\n", punqUtils.BytesToHumanReadable(result.Bytes), result.DownloadUrl)

// 	return result
// }

type K8sManagerUpgradeRequest struct {
	Command string `json:"command" validate:"required"` // complete helm command from platform ui
}

// func K8sManagerUpgradeRequestExample() K8sManagerUpgradeRequest {
// 	return K8sManagerUpgradeRequest{
// 		Command: `helm repo add mogenius https://helm.mogenius.com/public
// 		helm repo update
// 		helm upgrade mogenius mogenius/mogenius-platform -n mogenius \
// 		--set global.cluster_name="gcp2" \
// 		--set global.api_key="mo_e8a0ac85-c158-4d9d-83aa-d488218fc9f7_vlhqnlum2uh9q8kdhdmu" \
// 		--set global.namespace="mogenius" \
// 		--set k8smanager.enabled=true \
// 		--set metrics.enabled=false \
// 		--set traffic-collector.enabled=true \
// 		--set pod-stats-collector.enabled=true \
// 		--set ingress-nginx.enabled=true \
// 		--set certmanager.enabled=true \
// 		--set cert-manager.startupapicheck.enabled=false \
// 		--set certmanager.namespace="mogenius" \
// 		--set cert-manager.namespace="mogenius" \
// 		--set cert-manager.installCRDs=true`,
// 	}
// }

type ClusterHelmRequest struct {
	Namespace       string `json:"namespace" validate:"required"`
	NamespaceId     string `json:"namespaceId" validate:"required"`
	HelmRepoName    string `json:"helmRepoName" validate:"required"`
	HelmRepoUrl     string `json:"helmRepoUrl" validate:"required"`
	HelmReleaseName string `json:"helmReleaseName" validate:"required"`
	HelmChartName   string `json:"helmChartName" validate:"required"`
	HelmValues      string `json:"helmValues" validate:"required"`
}

func ClusterHelmRequestExample() ClusterHelmRequest {
	return ClusterHelmRequest{
		Namespace:       "mogenius",
		NamespaceId:     "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		HelmRepoUrl:     "https://charts.bitnami.com/bitnami",
		HelmReleaseName: "test-helm-release",
		HelmChartName:   "bitnami/nginx",
		HelmValues:      "",
	}
}

type ClusterHelmUninstallRequest struct {
	NamespaceId     string `json:"namespaceId" validate:"required"`
	HelmReleaseName string `json:"helmReleaseName" validate:"required"`
}

func ClusterHelmUninstallRequestExample() ClusterHelmUninstallRequest {
	return ClusterHelmUninstallRequest{
		NamespaceId:     "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		HelmReleaseName: "test-helm-release",
	}
}

type ClusterWriteConfigMap struct {
	Namespace string            `json:"namespace" validate:"required"`
	Name      string            `json:"name" validate:"required"`
	Labels    map[string]string `json:"labels" validate:"required"`
	Data      string            `json:"data" validate:"required"`
}

func ClusterWriteConfigMapExample() ClusterWriteConfigMap {
	return ClusterWriteConfigMap{
		Namespace: "mogenius",
		Name:      "my-funky-configmap",
		Labels: map[string]string{
			"app": "my-funky-app",
		},
		Data: "my-funky-data-yaml-string",
	}
}

type ClusterListWorkloads struct {
	Namespace     string `json:"namespace"`
	LabelSelector string `json:"labelSelector"`
	Prefix        string `json:"prefix"`
}

func ClusterListWorkloadsExample() ClusterListWorkloads {
	return ClusterListWorkloads{
		Namespace:     "mogenius",
		LabelSelector: "",
		Prefix:        "metal",
	}
}

type ClusterUpdateLocalTlsSecret struct {
	LocalTlsCrt string `json:"localTlsCrt" validate:"required"`
	LocalTlsKey string `json:"localTlsKey" validate:"required"`
}

func ClusterUpdateLocalTlsSecretExample() ClusterUpdateLocalTlsSecret {
	return ClusterUpdateLocalTlsSecret{
		LocalTlsCrt: "my-funky-crt",
		LocalTlsKey: "my-funky-key",
	}
}

type ClusterGetConfigMap struct {
	Namespace string `json:"namespace" validate:"required"`
	Name      string `json:"name" validate:"required"`
}

func ClusterGetConfigMapExample() ClusterGetConfigMap {
	return ClusterGetConfigMap{
		Namespace: "mogenius",
		Name:      "my-funky-configmap",
	}
}

type ClusterGetDeployment struct {
	Namespace string `json:"namespace" validate:"required"`
	Name      string `json:"name" validate:"required"`
}

func ClusterGetDeploymentExample() ClusterGetDeployment {
	return ClusterGetDeployment{
		Namespace: "mogenius",
		Name:      "my-funky-deployment",
	}
}

type ClusterGetPersistentVolume struct {
	Namespace string `json:"namespace" validate:"required"`
	Name      string `json:"name" validate:"required"`
}

func ClusterGetPersistentVolumeExample() ClusterGetPersistentVolume {
	return ClusterGetPersistentVolume{
		Namespace: "mogenius",
		Name:      "nfs-server-my-funky-nfs",
	}
}

type NfsStorageInstallRequest struct {
	ClusterProvider punqDtos.KubernetesProvider `json:"clusterProvider"`
}

func NfsStorageInstallRequestExample() NfsStorageInstallRequest {
	return NfsStorageInstallRequest{
		ClusterProvider: punqDtos.AKS,
	}
}

type NfsVolumeRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
	VolumeName    string `json:"volumeName" validate:"required"`
	SizeInGb      int    `json:"sizeInGb" validate:"required"`
}

func NfsVolumeRequestExample() NfsVolumeRequest {
	return NfsVolumeRequest{
		NamespaceName: "name",
		VolumeName:    "my-fancy-volume-name",
		SizeInGb:      10,
	}
}

type NfsVolumeStatsRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
	VolumeName    string `json:"volumeName" validate:"required"`
}

func NfsVolumeStatsRequestExample() NfsVolumeStatsRequest {
	return NfsVolumeStatsRequest{
		NamespaceName: "name",
		VolumeName:    "my-fancy-volume-name",
	}
}

type NfsNamespaceStatsRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
}

func NfsNamespaceStatsRequestExample() NfsNamespaceStatsRequest {
	return NfsNamespaceStatsRequest{
		NamespaceName: "test-bene-prod-a7fm72",
	}
}

type NfsVolumeStatsResponse struct {
	VolumeName string `json:"volumeName"`
	TotalBytes uint64 `json:"totalBytes"`
	FreeBytes  uint64 `json:"freeBytes"`
	UsedBytes  uint64 `json:"usedBytes"`
}

// token/accesskey/accesssecret can be generated using aws sts get-session-token | jq
// type NfsVolumeBackupRequest struct {
// 	NamespaceId        string `json:"namespaceId" validate:"required"`
// 	NamespaceName      string `json:"namespaceName" validate:"required"`
// 	VolumeName         string `json:"volumeName" validate:"required"`
// 	AwsAccessKeyId     string `json:"awsAccessKeyId"`     // TEMP Credentials. Not security relevant
// 	AwsSecretAccessKey string `json:"awsSecretAccessKey"` // TEMP Credentials. Not security relevant
// 	AwsSessionToken    string `json:"awsSessionToken"`    // TEMP Credentials. Not security relevant
// }

// func (s *NfsVolumeBackupRequest) AddSecretsToRedaction() {
// 	utils.AddSecret(&s.AwsAccessKeyId)
// 	utils.AddSecret(&s.AwsSecretAccessKey)
// 	utils.AddSecret(&s.AwsSessionToken)
// }

// func NfsVolumeBackupRequestExample() NfsVolumeBackupRequest {
// 	return NfsVolumeBackupRequest{
// 		NamespaceId:        "B0919ACB-92DD-416C-AF67-E59AD4B25265",
// 		NamespaceName:      "mogenius",
// 		VolumeName:         "my-fancy-volume-name",
// 		AwsAccessKeyId:     DEBUG_AWS_ACCESS_KEY_ID, // TEMP Credentials. Not security relevant
// 		AwsSecretAccessKey: DEBUG_AWS_SECRET_KEY,    // TEMP Credentials. Not security relevant
// 		AwsSessionToken:    DEBUG_AWS_TOKEN,         // TEMP Credentials. Not security relevant
// 	}
// }

// // token/accesskey/accesssecret can be generated using aws sts get-session-token | jq
// type NfsVolumeRestoreRequest struct {
// 	NamespaceId        string `json:"namespaceId" validate:"required"`
// 	NamespaceName      string `json:"namespaceName" validate:"required"`
// 	VolumeName         string `json:"volumeName" validate:"required"`
// 	BackupKey          string `json:"backupKey" validate:"required"`
// 	AwsAccessKeyId     string `json:"awsAccessKeyId"`     // TEMP Credentials. Not security relevant
// 	AwsSecretAccessKey string `json:"awsSecretAccessKey"` // TEMP Credentials. Not security relevant
// 	AwsSessionToken    string `json:"awsSessionToken"`    // TEMP Credentials. Not security relevant
// }

// func (s *NfsVolumeRestoreRequest) AddSecretsToRedaction() {
// 	utils.AddSecret(&s.AwsAccessKeyId)
// 	utils.AddSecret(&s.AwsSecretAccessKey)
// 	utils.AddSecret(&s.AwsSessionToken)
// }

// func NfsVolumeRestoreRequestExample() NfsVolumeRestoreRequest {
// 	return NfsVolumeRestoreRequest{
// 		NamespaceId:        "B0919ACB-92DD-416C-AF67-E59AD4B25265",
// 		NamespaceName:      "mogenius",
// 		VolumeName:         "my-fancy-volume-name",
// 		BackupKey:          "backup_my-fancy-volume-name_2023-04-11T13:45:00+02:00.zip",
// 		AwsAccessKeyId:     DEBUG_AWS_ACCESS_KEY_ID, // TEMP Credentials. Not security relevant
// 		AwsSecretAccessKey: DEBUG_AWS_SECRET_KEY,    // TEMP Credentials. Not security relevant
// 		AwsSessionToken:    DEBUG_AWS_TOKEN,         // TEMP Credentials. Not security relevant
// 	}
// }

// type NfsVolumeBackupResponse struct {
// 	VolumeName  string `json:"volumeName"`
// 	DownloadUrl string `json:"downloadUrl"`
// 	Bytes       int64  `json:"bytes"`
// 	Error       string `json:"error,omitempty"`
// }

// type NfsVolumeRestoreResponse struct {
// 	VolumeName string `json:"volumeName"`
// 	Message    string `json:"message"`
// 	Error      string `json:"error,omitempty"`
// }

// @TODO: add request/respionse example for nfs status
type NfsStatusRequest struct {
	Name             string `json:"name" validate:"required"`
	Namespace        string `json:"namespace"`
	StorageAPIObject string `json:"type" validate:"required"`
}

type NfsStatusResponse struct {
	VolumeName    string                `json:"volumeName"`
	NamespaceName string                `json:"namespaceName"`
	TotalBytes    uint64                `json:"totalBytes"`
	FreeBytes     uint64                `json:"freeBytes"`
	UsedBytes     uint64                `json:"usedBytes"`
	Status        VolumeStatusType      `json:"status"`
	Messages      []VolumeStatusMessage `json:"messages,omitempty"`
	UsedByPods    []string              `json:"usedByPods,omitempty"`
}

var keplerHostAndPort string = ""

var energyConsumptionCollectionInProgress bool = false

func EnergyConsumption() []structs.EnergyConsumptionResponse {
	if energyConsumptionCollectionInProgress {
		return structs.CurrentEnergyConsumptionResponse
	}

	if keplerHostAndPort == "" {
		keplerservice := mokubernetes.ServiceWithLabels("app.kubernetes.io/component=exporter,app.kubernetes.io/name=kepler", nil)
		if keplerservice != nil {
			keplerHostAndPort = fmt.Sprintf("%s:%d", keplerservice.Name, keplerservice.Spec.Ports[0].Port)
		} else {
			ServiceLogger.Errorf("EnergyConsumption Err: kepler service not found.")
			return structs.CurrentEnergyConsumptionResponse
		}
		// if utils.CONFIG.Misc.Stage == utils.STAGE_LOCAL {
		// 	ServiceLogger.Warning("OVERWRITTEN ACTUAL IP BECAUSE RUNNING IN LOCAL MODE! 192.168.178.132:9102")
		// 	keplerHostAndPort = "127.0.0.1:9102"
		// }
	}
	if structs.KeplerDaemonsetRunningSince == 0 {
		keplerPod := mokubernetes.KeplerPod()
		if keplerPod != nil && keplerPod.Status.StartTime != nil {
			structs.KeplerDaemonsetRunningSince = keplerPod.Status.StartTime.Time.Unix()
		}
	}

	go func() {
		energyConsumptionCollectionInProgress = true
		structs.CurrentEnergyConsumptionResponse = make([]structs.EnergyConsumptionResponse, structs.EnergyConsumptionResponseSize)
		for i := 0; i < structs.EnergyConsumptionResponseSize; i++ {
			// download the data
			response, err := http.Get(fmt.Sprintf("http://%s/metrics", keplerHostAndPort))
			if err != nil {
				ServiceLogger.Errorf("EnergyConsumption Err: %s", err.Error())
				return
			}
			defer response.Body.Close()
			data, err := io.ReadAll(response.Body)
			if err != nil {
				ServiceLogger.Errorf("EnergyConsumptionRead Err: %s", err.Error())
				return
			}

			// parse the data
			structs.CreateEnergyConsumptionResponse(string(data), i)
			time.Sleep(structs.EnergyConsumptionTimeInterval * time.Second)
		}
		energyConsumptionCollectionInProgress = false
	}()

	return structs.CurrentEnergyConsumptionResponse
}

func InstallTrafficCollector() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoUrl:     MogeniusHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameTrafficCollector,
		HelmChartName:   "mogenius/" + utils.HelmReleaseNameTrafficCollector,
		HelmValues: fmt.Sprintf(`global:
  namespace: %s
  stage: %s
`, utils.CONFIG.Kubernetes.OwnNamespace, utils.CONFIG.Misc.Stage),
	}
	return mokubernetes.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
}

func UpgradeTrafficCollector() (string, error) {
	r := mokubernetes.HelmReleaseUpgradeRequest{
		Namespace: utils.CONFIG.Kubernetes.OwnNamespace,
		Release:   utils.HelmReleaseNameTrafficCollector,
		Chart:     "mogenius/" + utils.HelmReleaseNameTrafficCollector,
		Values: fmt.Sprintf(`global:
  namespace: %s
  stage: %s
`, utils.CONFIG.Kubernetes.OwnNamespace, utils.CONFIG.Misc.Stage),
	}
	return mokubernetes.HelmReleaseUpgrade(r)
}

func InstallPodStatsCollector() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    "mogenius",
		HelmRepoUrl:     MogeniusHelmIndex,
		HelmReleaseName: utils.HelmReleaseNamePodStatsCollector,
		HelmChartName:   "mogenius/" + utils.HelmReleaseNamePodStatsCollector,
		HelmValues: fmt.Sprintf(`global:
  namespace: %s
  stage: %s
`, utils.CONFIG.Kubernetes.OwnNamespace, utils.CONFIG.Misc.Stage),
	}
	return mokubernetes.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
}

func UpgradePodStatsCollector() (string, error) {
	r := mokubernetes.HelmReleaseUpgradeRequest{
		Namespace: utils.CONFIG.Kubernetes.OwnNamespace,
		Release:   utils.HelmReleaseNamePodStatsCollector,
		Chart:     "mogenius/" + utils.HelmReleaseNamePodStatsCollector,
		Values: fmt.Sprintf(`global:
  namespace: %s
  stage: %s
`, utils.CONFIG.Kubernetes.OwnNamespace, utils.CONFIG.Misc.Stage),
	}
	return mokubernetes.HelmReleaseUpgrade(r)
}

func InstallMetricsServer() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    utils.HelmReleaseNameMetricsServer,
		HelmRepoUrl:     MetricsHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameMetricsServer,
		HelmChartName:   utils.HelmReleaseNameMetricsServer + "/" + utils.HelmReleaseNameMetricsServer,
		HelmValues: `args:
  - "--kubelet-insecure-tls"
  - "--secure-port=10250"
  - "--cert-dir=/tmp"
  - "--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname"
  - "--kubelet-use-node-status-port"
  - "--metric-resolution=15s"
`,
	}
	return mokubernetes.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
}

func UpgradeMetricsServer() (string, error) {
	r := mokubernetes.HelmReleaseUpgradeRequest{
		Namespace: "default",
		Release:   utils.HelmReleaseNameMetricsServer,
		Chart:     utils.HelmReleaseNameMetricsServer + "/" + utils.HelmReleaseNameMetricsServer,
		Values: `args:
  - "--kubelet-insecure-tls"
  - "--secure-port=10250"
  - "--cert-dir=/tmp"
  - "--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname"
  - "--kubelet-use-node-status-port"
  - "--metric-resolution=15s"
`,
	}
	return mokubernetes.HelmReleaseUpgrade(r)
}

func InstallIngressControllerTreafik() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    utils.HelmReleaseNameTraefik,
		HelmRepoUrl:     IngressControllerTraefikHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameTraefik,
		HelmChartName:   utils.HelmReleaseNameTraefik + "/" + utils.HelmReleaseNameTraefik,
		HelmValues:      "",
	}
	return mokubernetes.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
}

func UpgradeIngressControllerTreafik() (string, error) {
	r := mokubernetes.HelmReleaseUpgradeRequest{
		Namespace: "default",
		Release:   utils.HelmReleaseNameTraefik,
		Chart:     utils.HelmReleaseNameTraefik + "/" + utils.HelmReleaseNameTraefik,
	}
	return mokubernetes.HelmReleaseUpgrade(r)
}

func InstallCertManager() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    "jetstack",
		HelmRepoUrl:     CertManagerHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameCertManager,
		HelmChartName:   "jetstack/" + utils.HelmReleaseNameCertManager,
		HelmValues: fmt.Sprintf(`namespace: %s
startupapicheck:
  enabled: false
installCRDs: true
`, utils.CONFIG.Kubernetes.OwnNamespace),
	}
	return mokubernetes.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
}

func UpgradeCertManager() (string, error) {
	r := mokubernetes.HelmReleaseUpgradeRequest{
		Namespace: utils.CONFIG.Kubernetes.OwnNamespace,
		Release:   utils.HelmReleaseNameCertManager,
		Chart:     "jetstack/" + utils.HelmReleaseNameCertManager,
		Values:    "startupapicheck.enabled=false\ninstallCRDs=true",
	}
	return mokubernetes.HelmReleaseUpgrade(r)
}

func InstallContainerRegistry() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    "phntom",
		HelmRepoUrl:     ContainerRegistryHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameDistributionRegistry,
		HelmChartName:   "phntom/docker-registry",
		HelmValues:      "",
	}
	return mokubernetes.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
}

func InstallExternalSecrets() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    utils.HelmReleaseNameExternalSecrets,
		HelmRepoUrl:     ExternalSecretsHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameExternalSecrets,
		HelmChartName:   "external-secrets/external-secrets",
		HelmValues:      "",
	}
	return mokubernetes.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
}

func UpgradeContainerRegistry() (string, error) {
	r := mokubernetes.HelmReleaseUpgradeRequest{
		Namespace: utils.CONFIG.Kubernetes.OwnNamespace,
		Release:   utils.HelmReleaseNameDistributionRegistry,
		Chart:     "phntom/docker-registry",
	}
	return mokubernetes.HelmReleaseUpgrade(r)
}

func InstallMetalLb() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    utils.HelmReleaseNameMetalLb,
		HelmRepoUrl:     MetalLBHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameMetalLb,
		HelmChartName:   utils.HelmReleaseNameMetalLb + "/" + utils.HelmReleaseNameMetalLb,
		HelmValues:      "",
	}
	helmResultStr, err := mokubernetes.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
	if err == nil {
		for {
			// this is important because the control plane needs some time to make the CRDs available
			time.Sleep(1 * time.Second)
			err := mokubernetes.CreateYamlString(InstallAddressPool())
			if err != nil && !apierrors.IsAlreadyExists(err) {
				ServiceLogger.Errorf("Error installing metallb address pool: %s", err.Error())
			}
			if err != nil && apierrors.IsInternalError(err) {
				ServiceLogger.Infof("Control plane not ready. Waiting for metallb address pool installation ...")
			}
			if err == nil {
				break
			}
		}
	}
	return helmResultStr, err
}

func UpgradeMetalLb() (string, error) {
	r := mokubernetes.HelmReleaseUpgradeRequest{
		Namespace: utils.CONFIG.Kubernetes.OwnNamespace,
		Release:   utils.HelmReleaseNameMetalLb,
		Chart:     utils.HelmReleaseNameMetalLb + "/" + utils.HelmReleaseNameMetalLb,
	}
	return mokubernetes.HelmReleaseUpgrade(r)
}

func InstallKepler() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    utils.HelmReleaseNameKepler,
		HelmRepoUrl:     KeplerHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameKepler,
		HelmChartName:   utils.HelmReleaseNameKepler + "/" + utils.HelmReleaseNameKepler,
		HelmValues: fmt.Sprintf(`global:
  namespace: "%s"
extraEnvVars:
  EXPOSE_IRQ_COUNTER_METRICS: "false"
  EXPOSE_KUBELET_METRICS: "false"
  ENABLE_PROCESS_METRICS: "false"
`, utils.CONFIG.Kubernetes.OwnNamespace),
	}
	return mokubernetes.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
}

func UpgradeKepler() (string, error) {
	r := mokubernetes.HelmReleaseUpgradeRequest{
		Namespace: utils.CONFIG.Kubernetes.OwnNamespace,
		Chart:     utils.HelmReleaseNameKepler + "/" + utils.HelmReleaseNameKepler,
		Release:   utils.HelmReleaseNameKepler,
		Values: fmt.Sprintf(`global:
  namespace: "%s"
extraEnvVars:
  EXPOSE_IRQ_COUNTER_METRICS: "false"
  EXPOSE_KUBELET_METRICS: "false"
  ENABLE_PROCESS_METRICS: "false"
`, utils.CONFIG.Kubernetes.OwnNamespace),
	}
	return mokubernetes.HelmReleaseUpgrade(r)
}

func InstallClusterIssuer(email string, currentRetries int) (string, error) {
	time.Sleep(3 * time.Second) // wait for cert-manager to be ready
	maxRetries := 10
	if currentRetries >= maxRetries {
		return "", fmt.Errorf("No suitable Ingress Controller found. Please install Traefik or Nginx Ingress Controller first.")
	} else {
		ingType, err := punq.DetermineIngressControllerType(nil)
		if err != nil {
			ServiceLogger.Errorf("InstallClusterIssuer: Error determining ingress controller type: %s", err.Error())
		}
		if ingType == punq.TRAEFIK || ingType == punq.NGINX {
			r := ClusterHelmRequest{
				HelmRepoName:    "mogenius",
				HelmRepoUrl:     MogeniusHelmIndex,
				HelmReleaseName: utils.HelmReleaseNameClusterIssuer,
				HelmChartName:   "mogenius/mogenius-cluster-issuer",
				HelmValues: fmt.Sprintf(`global:
  clusterissuermail: "%s"
  ingressclass: "%s"
`, email, strings.ToLower(ingType.String())),
			}
			result, err := mokubernetes.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
			if err != nil {
				currentRetries++
				_, err := InstallClusterIssuer(email, currentRetries)
				if err != nil {
					ServiceLogger.Debugf("Error installing cluster issuer: %s", err.Error())
				}
			}
			return result, err
		}
		ServiceLogger.Infof("No suitable Ingress Controller found (%s). Retry in 3 seconds (%d/%d) ...", ingType.String(), currentRetries, maxRetries)
		currentRetries++
		return InstallClusterIssuer(email, currentRetries)
	}
}

func UninstallTrafficCollector() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmReleaseName: utils.HelmReleaseNameTrafficCollector,
	}
	return mokubernetes.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallPodStatsCollector() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmReleaseName: utils.HelmReleaseNamePodStatsCollector,
	}
	return mokubernetes.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallMetricsServer() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       "default",
		HelmReleaseName: utils.HelmReleaseNameMetricsServer,
	}
	return mokubernetes.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallIngressControllerTreafik() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       "default",
		HelmReleaseName: utils.HelmReleaseNameTraefik,
	}
	return mokubernetes.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallCertManager() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmReleaseName: utils.HelmReleaseNameCertManager,
	}
	return mokubernetes.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallContainerRegistry() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmReleaseName: utils.HelmReleaseNameDistributionRegistry,
	}
	return mokubernetes.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallExternalSecrets() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmReleaseName: utils.HelmReleaseNameExternalSecrets,
	}
	return mokubernetes.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallMetalLb() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmReleaseName: utils.HelmReleaseNameMetalLb,
	}
	return mokubernetes.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallKepler() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmReleaseName: utils.HelmReleaseNameKepler,
	}
	return mokubernetes.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallClusterIssuer() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmReleaseName: utils.HelmReleaseNameClusterIssuer,
	}
	return mokubernetes.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func InstallDefaultApplications() (string, string) {
	userApps := ""
	basicApps := `
# install kubectl
if command -v kubectl >/dev/null 2>&1; then
    echo "kubectl is installed. Skipping installation."
else
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/${GOARCH}/kubectl"
chmod +x kubectl
mv kubectl /usr/local/bin/kubectl
	echo "kubectl is installed. 🚀"
fi

# install popeye
if command -v popeye >/dev/null 2>&1; then
    echo "popeye is installed. Skipping installation."
else
	if [ "${GOARCH}" = "amd64" ]; then
		curl -fsSL -o popeye.tar.gz https://github.com/derailed/popeye/releases/download/v0.11.1/popeye_Linux_x86_64.tar.gz;
	elif [ "${GOARCH}" = "arm64" ]; then
		curl -fsSL -o popeye.tar.gz https://github.com/derailed/popeye/releases/download/v0.11.1/popeye_Linux_arm64.tar.gz;
	elif [ "${GOARCH}" = "arm" ]; then
		curl -fsSL -o popeye.tar.gz https://github.com/derailed/popeye/releases/download/v0.11.1/popeye_Linux_arm.tar.gz;
	else
		echo "Unsupported architecture";
	fi
	tar -xf popeye.tar.gz popeye
	chmod +x popeye
	mv popeye /usr/local/bin/popeye
	rm popeye.tar.gz
	echo "popeye is installed. 🚀"
fi

# install grype
if type grype >/dev/null 2>&1; then
    echo "grype is installed. Skipping installation."
else
	curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin
	echo "grype is installed. 🚀"
fi

# install dive
if type dive >/dev/null 2>&1; then
    echo "dive is installed. Skipping installation."
else
	DIVE_VERSION=$(curl -sL "https://api.github.com/repos/wagoodman/dive/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
	if [ "${GOARCH}" = "amd64" ]; then
		curl -o dive.tar.gz -L https://github.com/wagoodman/dive/releases/download/v${DIVE_VERSION}/dive_${DIVE_VERSION}_linux_amd64.tar.gz
	elif [ "${GOARCH}" = "arm64" ]; then
		curl -o dive.tar.gz -L https://github.com/wagoodman/dive/releases/download/v${DIVE_VERSION}/dive_${DIVE_VERSION}_linux_arm64.tar.gz
	else
		echo "Unsupported architecture";
	fi
	tar -xf dive.tar.gz dive
	chmod +x dive
	mv dive /usr/local/bin/dive
	rm dive.tar.gz
	echo "dive is installed. 🚀"
fi

# install trivy
if type trivy >/dev/null 2>&1; then
    echo "trivy is installed. Skipping installation."
else
	curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin latest
	echo "trivy is installed. 🚀"
fi

# install k9s
if type k9s >/dev/null 2>&1; then
    echo "k9s is installed. Skipping installation."
else
	K9S_VERSION=$(curl -sL "https://api.github.com/repos/derailed/k9s/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
	if [ "${GOARCH}" = "amd64" ]; then
		curl -o k9s.tar.gz -L https://github.com/derailed/k9s/releases/download/v${K9S_VERSION}/k9s_Linux_amd64.tar.gz
	elif [ "${GOARCH}" = "arm64" ]; then
		curl -o k9s.tar.gz -L https://github.com/derailed/k9s/releases/download/v${K9S_VERSION}/k9s_Linux_arm64.tar.gz
	elif [ "${GOARCH}" = "arm" ]; then
		curl -o k9s.tar.gz -L https://github.com/derailed/k9s/releases/download/v${K9S_VERSION}/k9s_Linux_armv7.tar.gz
	else
		echo "Unsupported architecture";
	fi
	tar -xf k9s.tar.gz k9s
	chmod +x k9s
	mv k9s /usr/local/bin/k9s
	rm k9s.tar.gz
	echo "k9s is installed. 🚀"
fi

# create kubeconfig
cat <<EOF > kubeconfig.tmpl
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: @@CACRT@@
    server: https://@@IP@@:6443
  name: local
contexts:
- context:
    cluster: local
    user: mogenius
    namespace: mogenius
  name: mogenius
current-context: mogenius
kind: Config
preferences: {}
users:
- name: mogenius
  user:
    token: @@TOKEN@@
EOF
cat kubeconfig.tmpl | sed -e s/@@CACRT@@/$(echo -n "$(cat /var/run/secrets/kubernetes.io/serviceaccount/ca.crt)" | base64 | tr -d '\n')/| sed -e s/@@TOKEN@@/$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)/ | sed -e s/@@IP@@/$(kubectl get nodes -o json | jq '.items[0].status.addresses[0].address' | sed -e s/\"//g)/ > kubeconfig.yaml
echo "KUBECONFIG created. 🚀"
`
	defaultAppsConfigmap := punq.ConfigMapFor(utils.CONFIG.Kubernetes.OwnNamespace, utils.MOGENIUS_CONFIGMAP_DEFAULT_APPS_NAME, false, nil)
	if defaultAppsConfigmap != nil {
		if installCommands, exists := defaultAppsConfigmap.Data["install-commands"]; exists {
			userApps = installCommands
		}
	}

	return basicApps, userApps
}

func InstallAddressPool() string {
	return `apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: mo-pool
  namespace: mogenius
spec:
  addresses:
  - 192.168.66.1-192.168.66.50
  - fc00:f853:0ccd:e797::/124`
}

func getCurrentTrafficCollectorVersion() (string, error) {
	data, err := utils.GetVersionData(utils.CONFIG.Misc.HelmIndex)
	if err != nil {
		return "NO_VERSION_FOUND", err
	}
	trafficCollector := data.Entries[utils.HelmReleaseNameTrafficCollector]
	trafficResult := "NO_VERSION_FOUND"
	if len(trafficCollector) > 0 {
		trafficResult = trafficCollector[0].Version
	}
	return trafficResult, nil
}

func getCurrentPodStatsCollectorVersion() (string, error) {
	data, err := utils.GetVersionData(utils.CONFIG.Misc.HelmIndex)
	if err != nil {
		return "NO_VERSION_FOUND", err
	}
	podstatsCollector := data.Entries[utils.HelmReleaseNamePodStatsCollector]
	podstatsResult := "NO_VERSION_FOUND"
	if len(podstatsCollector) > 0 {
		podstatsResult = podstatsCollector[0].Version
	}
	return podstatsResult, nil
}

func getMostCurrentHelmChartVersion(url string, chartname string) string {
	url = addIndexYAMLtoURL(url)
	data, err := utils.GetVersionData(url)
	if err != nil {
		ServiceLogger.Errorf("Error getting helm chart version (%s/%s): %s", url, chartname, err)
		return ""
	}
	chartsArray := data.Entries[chartname]
	result := "NO_VERSION_FOUND"
	if len(chartsArray) > 0 {
		result = chartsArray[0].Version
	}

	return result
}

func addIndexYAMLtoURL(url string) string {
	if !strings.HasSuffix(url, "index.yaml") {
		// Check if the URL ends with a slash; if not, add one.
		if !strings.HasSuffix(url, "/") {
			url += "/"
		}
		url += "index.yaml"
	}
	return url
}
