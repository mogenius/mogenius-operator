package services

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"mogenius-k8s-manager/db"
	"mogenius-k8s-manager/kubernetes"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	punqDtos "github.com/mogenius/punq/dtos"
	punq "github.com/mogenius/punq/kubernetes"
	punqStructs "github.com/mogenius/punq/structs"
	punqUtils "github.com/mogenius/punq/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/shirou/gopsutil/v3/disk"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const BUCKETNAME = "mogenius-backup"
const DEBUG_AWS_ACCESS_KEY_ID = "ASIAZNXZOUKFCEK3TPOL"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         // TEMP Credentials. Not security relevant
const DEBUG_AWS_SECRET_KEY = "xTsv35O30o87m6DuWOscHpKbxbXJeo0vS9iFkGwY"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        // TEMP Credentials. Not security relevant
const DEBUG_AWS_TOKEN = "IQoJb3JpZ2luX2VjELz//////////wEaDGV1LWNlbnRyYWwtMSJHMEUCIQCdZPuNWJCNJOSbMBhRtQb8W/0ylEV/ge1fiWFWgmD8ywIgEy/X2IAopx69LIGQQS+c2pRo4cSRFSslylRs8J7eUawq3AEIpf//////////ARABGgw2NDc5ODk0Njk4MzQiDPCjbP1jO5NAL96r2yqwAa3cCaeF8s1x2Zs8vAU+gRfK/tUZac8XjnJsjIxbmikiDPLuyPonsymuAd9D4ISK4fLeUU+BUU899fLHjIa2bWXRx1OrmGPIK3d/qBZF3pUPRid5AV8IRMiiP2sMI5RZzKpJfuWHH5WLknw0P7HYvusUlgAR4AgqPabHAE0c2Q1qaplJQrBXGeXCtMzs386OSPBQGogeBGn9Eu/l8QpySaA6RE3KgwvRELcvacMtmcdxMJ2H1aEGOpgBFNijTvMHK7D1pSOmvDfx9p9wSHZT/Red/G1CFWjUtV2H9+H4N+qrZTX2A4I9UGVEc+UlAQlIOAXPli2WTSPOdB7txbKsozU1YPVbi/gSZFXmGy8EFJml3bkg4HqSlozHLB/f1Ib81n9eWoUPOXp5SMwn6izW4ZZB3g8QSV6btOx2+s+Pm4BsLHICMhg3Rr0KI6ThnNhXcj8=" // TEMP Credentials. Not security relevant

const (
	NameMetricsServer             = "Metrics Server"
	NameMetalLB                   = "MetalLB (LoadBalancer)"
	NameIngressController         = "Ingress Controller"
	NameLocalDevSetup             = "Local Dev Setup"
	NameInternalContainerRegistry = "Internal Container Registry"
	NameClusterIssuerResource     = "letsencrypt-cluster-issuer"
	NameKepler                    = "Kepler"
	NameNfsStorageClass           = "nfs-storageclass"
)

const (
	MetricsHelmIndex                  = "https://kubernetes-sigs.github.io/metrics-server"
	IngressControllerTraefikHelmIndex = "https://traefik.github.io/charts"
	ContainerRegistryHelmIndex        = "https://phntom.kix.co.il/charts"
	CertManagerHelmIndex              = "https://charts.jetstack.io"
	KeplerHelmIndex                   = "https://sustainable-computing-io.github.io/kepler-helm-chart"
	MetalLBHelmIndex                  = "https://metallb.github.io/metallb"
	MogeniusHelmIndex                 = "https://helm.mogenius.com/public"
)

func UpgradeK8sManager(r K8sManagerUpgradeRequest) structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Upgrade mogenius platform", "UPGRADE", nil, nil)
	job.Start()
	job.AddCmd(mokubernetes.UpgradeMyself(&job, r.Command, &wg))
	wg.Wait()
	job.Finish()
	return job
}

func InstallHelmChart(r ClusterHelmRequest) structs.Job {
	// var wg sync.WaitGroup

	job := structs.CreateJob("Install Helm Chart "+r.HelmReleaseName, r.NamespaceId, nil, nil)
	job.Start()
	// TODO: make it working again. was disabled due to refactoring
	// job.AddCmd(mokubernetes.CreateHelmChartCmd(&job, r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, &wg))
	job.Finish()
	return job
}

func DeleteHelmChart(r ClusterHelmUninstallRequest) structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Delete Helm Chart "+r.HelmReleaseName, r.NamespaceId, nil, nil)
	job.Start()
	job.AddCmd(mokubernetes.DeleteHelmChart(&job, r.HelmReleaseName, &wg))
	wg.Wait()
	job.Finish()
	return job
}

func CreateMogeniusNfsVolume(r NfsVolumeRequest) structs.DefaultResponse {
	var wg sync.WaitGroup
	job := structs.CreateJob("Create mogenius nfs-volume.", r.NamespaceId, &r.NamespaceId, nil)
	job.Start()
	// FOR K8SMANAGER
	job.AddCmd(mokubernetes.CreateMogeniusNfsServiceSync(&job, r.NamespaceName, r.VolumeName))
	job.AddCmd(mokubernetes.CreateMogeniusNfsPersistentVolumeClaim(&job, r.NamespaceName, r.VolumeName, r.SizeInGb, &wg))
	job.AddCmd(mokubernetes.CreateMogeniusNfsDeployment(&job, r.NamespaceName, r.VolumeName, &wg))
	// FOR SERVICES THAT WANT TO MOUNT
	job.AddCmd(mokubernetes.CreateMogeniusNfsPersistentVolumeForService(&job, r.NamespaceName, r.VolumeName, r.SizeInGb, &wg))
	job.AddCmd(mokubernetes.CreateMogeniusNfsPersistentVolumeClaimForService(&job, r.NamespaceName, r.VolumeName, r.SizeInGb, &wg))
	wg.Wait()
	job.Finish()

	nfsService := mokubernetes.ServiceForNfsVolume(r.NamespaceName, r.VolumeName)
	mokubernetes.Mount(r.NamespaceName, r.VolumeName, nfsService)

	return job.DefaultReponse()
}

func DeleteMogeniusNfsVolume(r NfsVolumeRequest) structs.DefaultResponse {
	var wg sync.WaitGroup
	job := structs.CreateJob("Delete mogenius nfs-volume.", r.NamespaceId, &r.NamespaceId, nil)
	job.Start()
	// FOR K8SMANAGER
	job.AddCmd(mokubernetes.DeleteMogeniusNfsDeployment(&job, r.NamespaceName, r.VolumeName, &wg))
	job.AddCmd(mokubernetes.DeleteMogeniusNfsService(&job, r.NamespaceName, r.VolumeName, &wg))
	job.AddCmd(mokubernetes.DeleteMogeniusNfsPersistentVolumeClaim(&job, r.NamespaceName, r.VolumeName, &wg))
	// FOR SERVICES THAT WANT TO MOUNT
	job.AddCmd(mokubernetes.DeleteMogeniusNfsPersistentVolumeForService(&job, r.VolumeName, r.NamespaceName, &wg))
	job.AddCmd(mokubernetes.DeleteMogeniusNfsPersistentVolumeClaimForService(&job, r.NamespaceName, r.VolumeName, &wg))
	wg.Wait()
	job.Finish()

	mokubernetes.Umount(r.NamespaceName, r.VolumeName)

	return job.DefaultReponse()
}

func StatsMogeniusNfsVolume(r NfsVolumeStatsRequest) NfsVolumeStatsResponse {
	result := NfsVolumeStatsResponse{
		VolumeName: r.VolumeName,
		FreeBytes:  0,
		UsedBytes:  0,
		TotalBytes: 0,
	}

	mountPath := utils.MountPath(r.NamespaceName, r.VolumeName, "/")
	usage, err := disk.Usage(mountPath)
	if err != nil {
		logger.Log.Errorf("StatsMogeniusNfsVolume Err: %s %s", mountPath, err.Error())
		return result
	} else {
		result.FreeBytes = usage.Free
		result.UsedBytes = usage.Used
		result.TotalBytes = usage.Total
	}
	logger.Log.Infof("💾: '%s' -> %s / %s (%s)", mountPath, punqUtils.BytesToHumanReadable(int64(result.UsedBytes)), punqUtils.BytesToHumanReadable(int64(result.TotalBytes)), fmt.Sprintf("%.1f%%", usage.UsedPercent))
	return result
}

func StatsMogeniusNfsNamespace(r NfsNamespaceStatsRequest) []NfsVolumeStatsResponse {
	result := []NfsVolumeStatsResponse{}

	if r.NamespaceName == "null" || r.NamespaceName == "" {
		logger.Log.Errorf("StatsMogeniusNfsNamespace Err: namespaceName cannot be null or empty.")
		return result
	}

	// get all pvc for single namespace
	pvcs := punq.AllPersistentVolumeClaims(r.NamespaceName, nil)

	for _, pvc := range pvcs {
		// remove podname "nfs-server-pod-"
		pvc.Name = strings.Replace(pvc.Name, fmt.Sprintf("%s-", utils.CONFIG.Misc.NfsPodPrefix), "", 1)

		entry := NfsVolumeStatsResponse{
			VolumeName: pvc.Name,
			FreeBytes:  0,
			UsedBytes:  0,
			TotalBytes: 0,
		}

		mountPath := utils.MountPath(r.NamespaceName, pvc.Name, "/")
		usage, err := disk.Usage(mountPath)
		if err != nil {
			logger.Log.Errorf("StatsMogeniusNfsNamespace Err: %s %s", mountPath, err.Error())
			continue
		} else {
			entry.FreeBytes = usage.Free
			entry.UsedBytes = usage.Used
			entry.TotalBytes = usage.Total
		}
		logger.Log.Infof("💾: '%s' -> %s / %s (%s)", mountPath, punqUtils.BytesToHumanReadable(int64(entry.UsedBytes)), punqUtils.BytesToHumanReadable(int64(entry.TotalBytes)), fmt.Sprintf("%.1f%%", usage.UsedPercent))
		result = append(result, entry)
	}
	return result
}

func BackupMogeniusNfsVolume(r NfsVolumeBackupRequest) NfsVolumeBackupResponse {
	result := NfsVolumeBackupResponse{
		VolumeName:  r.VolumeName,
		DownloadUrl: "",
	}

	var wg sync.WaitGroup
	job := structs.CreateJob("Create nfs-volume backup.", r.NamespaceId, nil, nil)
	job.Start()

	mountPath := utils.MountPath(r.NamespaceName, r.VolumeName, "")

	result = ZipDirAndUploadToS3(mountPath, fmt.Sprintf("backup_%s_%s.zip", r.VolumeName, time.Now().Format(time.RFC3339)), result, r.AwsAccessKeyId, r.AwsSecretAccessKey, r.AwsSessionToken)
	if result.Error != "" {
		job.State = punqStructs.JobStateFailed
	}

	wg.Wait()
	job.Finish()
	return result
}

func RestoreMogeniusNfsVolume(r NfsVolumeRestoreRequest) NfsVolumeRestoreResponse {
	result := NfsVolumeRestoreResponse{
		VolumeName: r.VolumeName,
		Message:    "",
	}

	var wg sync.WaitGroup
	job := structs.CreateJob("Restore nfs-volume backup.", r.NamespaceId, nil, nil)
	job.Start()

	result = UnzipAndReplaceFromS3(r.NamespaceName, r.VolumeName, r.BackupKey, result, r.AwsAccessKeyId, r.AwsSecretAccessKey, r.AwsSessionToken)
	if result.Error != "" {
		job.State = punqStructs.JobStateFailed
	}

	wg.Wait()
	job.Finish()
	return result
}

func UnzipAndReplaceFromS3(namespaceName string, volumeName string, BackupKey string, result NfsVolumeRestoreResponse, accessKeyId string, secretAccessKey string, token string) NfsVolumeRestoreResponse {
	// Set up an AWS session
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String("eu-central-1"),
		Credentials: credentials.NewStaticCredentials(accessKeyId, secretAccessKey, token),
	}))

	// Download the zip file from S3
	downloader := s3manager.NewDownloader(sess)
	buffer := &aws.WriteAtBuffer{}
	downloadedBytes, err := downloader.Download(buffer, &s3.GetObjectInput{
		Bucket: aws.String(BUCKETNAME),
		Key:    aws.String(BackupKey),
	})
	if err != nil {
		logger.Log.Errorf("s3 Download error: %s", err.Error())
		result.Error = err.Error()
		return result
	}

	// Replace files with downloaded data
	r, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(len(buffer.Bytes())))
	if err != nil {
		panic(err)
	}

	mountPath := utils.MountPath(namespaceName, volumeName, "")
	// TODO XXX REMOVE
	mountPath = fmt.Sprintf("%s/restore", mountPath)
	err = os.MkdirAll(mountPath, 0755)
	if err != nil {
		logger.Log.Fatal(err)
	}

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			logger.Log.Error(err)
		}
		defer rc.Close()

		// Create the destination file
		destFilepath := fmt.Sprintf("%s/%s", mountPath, f.Name)
		destFile, err := os.Create(destFilepath)
		if err != nil {
			logger.Log.Error(err)
		}
		defer destFile.Close()

		// Copy the contents of the source file to the destination file
		_, err = io.Copy(destFile, rc)
		if err != nil {
			logger.Log.Error(err)
		}

		// Print the name of the unzipped file
		if utils.CONFIG.Misc.Debug {
			logger.Log.Infof("Unzipped file: %s\n", destFilepath)
		}
	}

	msg := fmt.Sprintf("Successfully restored volume (%s) from S3!\n", punqUtils.BytesToHumanReadable(downloadedBytes))
	logger.Log.Info(msg)
	result.Message = msg

	return result
}

func ZipDirAndUploadToS3(directoryToZip string, targetFileName string, result NfsVolumeBackupResponse, accessKeyId string, secretAccessKey string, token string) NfsVolumeBackupResponse {
	// Set up an AWS session
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String("eu-central-1"),
		Credentials: credentials.NewStaticCredentials(accessKeyId, secretAccessKey, token),
	}))

	// Create a zip archive buffer
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Add all files in a directory to the archive
	err := filepath.Walk(directoryToZip, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		fileBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(directoryToZip, path)
		if err != nil {
			return err
		}

		zipFile, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		_, err = io.Copy(zipFile, bytes.NewReader(fileBytes))
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		logger.Log.Errorf("s3 walk files error: %s", err.Error())
		result.Error = err.Error()
		return result
	}

	// Close the zip archive
	err = zipWriter.Close()
	if err != nil {
		logger.Log.Errorf("s3 zip error: %s", err.Error())
		result.Error = err.Error()
		return result
	}

	// Upload the zip file to S3
	s3svc := s3.New(sess)
	_, err = s3svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(BUCKETNAME),     // Replace with your S3 bucket name
		Key:    aws.String(targetFileName), // Replace with the name you want to give the zip file in S3
		Body:   bytes.NewReader(buf.Bytes()),
	})
	if err != nil {
		logger.Log.Errorf("s3 Send error: %s", err.Error())
		result.Error = err.Error()
		return result
	}

	// Get the uploaded object and presign it.
	req, _ := s3svc.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(BUCKETNAME),
		Key:    aws.String(targetFileName),
	})
	url, err := req.Presign(15 * time.Minute)
	if err != nil {
		logger.Log.Errorf("s3 presign error: %s", err.Error())
		result.Error = err.Error()
		return result
	}
	headObj, err := s3svc.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(BUCKETNAME),
		Key:    aws.String(targetFileName),
	})
	if err != nil {
		logger.Log.Errorf("s3 headobject error: %s", err.Error())
		result.Error = err.Error()
		return result
	}

	result.DownloadUrl = url
	if headObj != nil {
		result.Bytes = *headObj.ContentLength
	}

	logger.Log.Infof("Successfully uploaded zip file (%s) to S3! -> %s\n", punqUtils.BytesToHumanReadable(result.Bytes), result.DownloadUrl)

	return result
}

type K8sManagerUpgradeRequest struct {
	Command string `json:"command" validate:"required"` // complete helm command from platform ui
}

func K8sManagerUpgradeRequestExample() K8sManagerUpgradeRequest {
	return K8sManagerUpgradeRequest{
		Command: `helm repo add mogenius https://helm.mogenius.com/public
		helm repo update
		helm upgrade mogenius mogenius/mogenius-platform -n mogenius \
		--set global.cluster_name="gcp2" \
		--set global.api_key="mo_e8a0ac85-c158-4d9d-83aa-d488218fc9f7_vlhqnlum2uh9q8kdhdmu" \
		--set global.namespace="mogenius" \
		--set k8smanager.enabled=true \
		--set metrics.enabled=false \
		--set traffic-collector.enabled=true \
		--set pod-stats-collector.enabled=true \
		--set ingress-nginx.enabled=true \
		--set certmanager.enabled=true \
		--set cert-manager.startupapicheck.enabled=false \
		--set certmanager.namespace="mogenius" \
		--set cert-manager.namespace="mogenius" \
		--set cert-manager.installCRDs=true`,
	}
}

type ClusterHelmRequest struct {
	Namespace       string               `json:"namespace" validate:"required"`
	NamespaceId     string               `json:"namespaceId" validate:"required"`
	HelmRepoName    string               `json:"helmRepoName" validate:"required"`
	HelmRepoUrl     string               `json:"helmRepoUrl" validate:"required"`
	HelmReleaseName string               `json:"helmReleaseName" validate:"required"`
	HelmChartName   string               `json:"helmChartName" validate:"required"`
	HelmFlags       string               `json:"helmFlags" validate:"required"`
	HelmTask        structs.HelmTaskEnum `json:"helmTask" validate:"required"`
}

func ClusterHelmRequestExample() ClusterHelmRequest {
	return ClusterHelmRequest{
		Namespace:       "mogenius",
		NamespaceId:     "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		HelmRepoName:    "bitnami",
		HelmRepoUrl:     "https://charts.bitnami.com/bitnami",
		HelmReleaseName: "test-helm-release",
		HelmChartName:   "bitnami/nginx",
		HelmFlags:       "",
		HelmTask:        structs.HelmInstall,
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
	NamespaceId   string `json:"namespaceId" validate:"required"`
	NamespaceName string `json:"namespaceName" validate:"required"`
	VolumeName    string `json:"volumeName" validate:"required"`
	SizeInGb      int    `json:"sizeInGb" validate:"required"`
}

func NfsVolumeRequestExample() NfsVolumeRequest {
	return NfsVolumeRequest{
		NamespaceId:   "B0919ACB-92DD-416C-AF67-E59AD4B25265",
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
type NfsVolumeBackupRequest struct {
	NamespaceId        string `json:"namespaceId" validate:"required"`
	NamespaceName      string `json:"namespaceName" validate:"required"`
	VolumeName         string `json:"volumeName" validate:"required"`
	AwsAccessKeyId     string `json:"awsAccessKeyId"`     // TEMP Credentials. Not security relevant
	AwsSecretAccessKey string `json:"awsSecretAccessKey"` // TEMP Credentials. Not security relevant
	AwsSessionToken    string `json:"awsSessionToken"`    // TEMP Credentials. Not security relevant
}

func NfsVolumeBackupRequestExample() NfsVolumeBackupRequest {
	return NfsVolumeBackupRequest{
		NamespaceId:        "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		NamespaceName:      "mogenius",
		VolumeName:         "my-fancy-volume-name",
		AwsAccessKeyId:     DEBUG_AWS_ACCESS_KEY_ID, // TEMP Credentials. Not security relevant
		AwsSecretAccessKey: DEBUG_AWS_SECRET_KEY,    // TEMP Credentials. Not security relevant
		AwsSessionToken:    DEBUG_AWS_TOKEN,         // TEMP Credentials. Not security relevant
	}
}

// token/accesskey/accesssecret can be generated using aws sts get-session-token | jq
type NfsVolumeRestoreRequest struct {
	NamespaceId        string `json:"namespaceId" validate:"required"`
	NamespaceName      string `json:"namespaceName" validate:"required"`
	VolumeName         string `json:"volumeName" validate:"required"`
	BackupKey          string `json:"backupKey" validate:"required"`
	AwsAccessKeyId     string `json:"awsAccessKeyId"`     // TEMP Credentials. Not security relevant
	AwsSecretAccessKey string `json:"awsSecretAccessKey"` // TEMP Credentials. Not security relevant
	AwsSessionToken    string `json:"awsSessionToken"`    // TEMP Credentials. Not security relevant
}

func NfsVolumeRestoreRequestExample() NfsVolumeRestoreRequest {
	return NfsVolumeRestoreRequest{
		NamespaceId:        "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		NamespaceName:      "mogenius",
		VolumeName:         "my-fancy-volume-name",
		BackupKey:          "backup_my-fancy-volume-name_2023-04-11T13:45:00+02:00.zip",
		AwsAccessKeyId:     DEBUG_AWS_ACCESS_KEY_ID, // TEMP Credentials. Not security relevant
		AwsSecretAccessKey: DEBUG_AWS_SECRET_KEY,    // TEMP Credentials. Not security relevant
		AwsSessionToken:    DEBUG_AWS_TOKEN,         // TEMP Credentials. Not security relevant
	}
}

type NfsVolumeBackupResponse struct {
	VolumeName  string `json:"volumeName"`
	DownloadUrl string `json:"downloadUrl"`
	Bytes       int64  `json:"bytes"`
	Error       string `json:"error,omitempty"`
}

type NfsVolumeRestoreResponse struct {
	VolumeName string `json:"volumeName"`
	Message    string `json:"message"`
	Error      string `json:"error,omitempty"`
}

var keplerHostAndPort string = ""

var energyConsumptionCollectionInProgress bool = false

func EnergyConsumption() []structs.EnergyConsumptionResponse {
	if energyConsumptionCollectionInProgress {
		return structs.CurrentEnergyConsumptionResponse
	}

	if keplerHostAndPort == "" {
		keplerservice := kubernetes.ServiceWithLabels("app.kubernetes.io/component=exporter,app.kubernetes.io/name=kepler", nil)
		if keplerservice != nil {
			keplerHostAndPort = fmt.Sprintf("%s:%d", keplerservice.Name, keplerservice.Spec.Ports[0].Port)
		} else {
			logger.Log.Errorf("EnergyConsumption Err: kepler service not found.")
			return structs.CurrentEnergyConsumptionResponse
		}
		// if utils.CONFIG.Misc.Stage == utils.STAGE_LOCAL {
		// 	logger.Log.Warning("OVERWRITTEN ACTUAL IP BECAUSE RUNNING IN LOCAL MODE! 192.168.178.132:9102")
		// 	keplerHostAndPort = "127.0.0.1:9102"
		// }
	}
	if structs.KeplerDaemonsetRunningSince == 0 {
		keplerPod := kubernetes.KeplerPod()
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
				logger.Log.Errorf("EnergyConsumption Err: %s", err.Error())
				return
			}
			defer response.Body.Close()
			data, err := io.ReadAll(response.Body)
			if err != nil {
				logger.Log.Errorf("EnergyConsumptionRead Err: %s", err.Error())
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

func SystemCheck() punq.SystemCheckResponse {
	entries := punq.SystemCheck()

	trafficCollectorNewestVersion, podstatsCollectorNewestVersion, err := getCurrentTrafficCollectorAndPodStatsVersion()
	if err != nil {
		logger.Log.Errorf("getCurrentTrafficCollectorVersion Err: %s", err.Error())
	}

	dockerResult, dockerOutput, dockerErr := IsDockerInstalled()
	kubeCtlMsg := punq.StatusMessage(dockerErr, "If docker is missing in this image, we are screwed ;-)", dockerOutput)
	entries = append(entries, punq.CreateSystemCheckEntry("docker", dockerResult, kubeCtlMsg, "", true, false, dockerOutput, ""))

	certManagerVersion, certManagerInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameCertManager)
	certManagerMsg := fmt.Sprintf("%s (Version: %s) is installed.", utils.HelmReleaseNameCertManager, certManagerVersion)
	if certManagerInstalledErr != nil {
		certManagerMsg = fmt.Sprintf("%s is not installed.\nTo create ssl certificates you need to install this component.", utils.HelmReleaseNameCertManager)
	}
	certMgrDescription := "Install the cert-manager to automatically issue Let's Encrypt certificates to your services."
	currentCertManagerVersion := getMostCurrentHelmChartVersion(CertManagerHelmIndex, utils.HelmReleaseNameCertManager)
	certMgrEntry := punq.CreateSystemCheckEntry(utils.HelmReleaseNameCertManager, certManagerInstalledErr == nil, certManagerMsg, certMgrDescription, false, true, certManagerVersion, currentCertManagerVersion)
	certMgrEntry.InstallPattern = PAT_INSTALL_CERT_MANAGER
	certMgrEntry.UninstallPattern = PAT_UNINSTALL_CERT_MANAGER
	certMgrEntry.UpgradePattern = PAT_UPGRADE_CERT_MANAGER
	certMgrEntry.Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameCertManager)
	entries = append(entries, certMgrEntry)

	_, clusterIssuerInstalledErr := punq.GetClusterIssuer(NameClusterIssuerResource, nil)
	clusterIssuerMsg := fmt.Sprintf("%s is installed.", NameClusterIssuerResource)
	if clusterIssuerInstalledErr != nil {
		clusterIssuerMsg = fmt.Sprintf("%s is not installed.\nTo issue ssl certificates you need to install this component.", NameClusterIssuerResource)
	}
	clusterIssuerDescription := "Responsible for signing certificates."
	clusterIssuerEntry := punq.CreateSystemCheckEntry(utils.HelmReleaseNameClusterIssuer, clusterIssuerInstalledErr == nil, clusterIssuerMsg, clusterIssuerDescription, false, true, "", "")
	clusterIssuerEntry.InstallPattern = PAT_INSTALL_CLUSTER_ISSUER
	clusterIssuerEntry.UninstallPattern = PAT_UNINSTALL_CLUSTER_ISSUER
	clusterIssuerEntry.Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameClusterIssuer)
	entries = append(entries, clusterIssuerEntry)

	trafficCollectorVersion, trafficCollectorInstalledErr := punq.IsDaemonSetInstalled(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameTrafficCollector)
	if trafficCollectorVersion == "" && trafficCollectorInstalledErr == nil {
		trafficCollectorVersion = "6.6.6" // flag local version without tag
	}
	trafficMsg := fmt.Sprintf("%s (Version: %s) is installed.", utils.HelmReleaseNameTrafficCollector, trafficCollectorVersion)
	if trafficCollectorInstalledErr != nil {
		trafficMsg = fmt.Sprintf("%s is not installed.\nTo gather traffic information you need to install this component.", utils.HelmReleaseNameTrafficCollector)
	}
	trafficDescpription := fmt.Sprintf("Collects and exposes detailed traffic data for your mogenius services for better monitoring. (Installed: %s | Available: %s)", trafficCollectorVersion, trafficCollectorNewestVersion)
	trafficEntry := punq.CreateSystemCheckEntry(utils.HelmReleaseNameTrafficCollector, trafficCollectorInstalledErr == nil, trafficMsg, trafficDescpription, false, true, trafficCollectorVersion, trafficCollectorNewestVersion)
	trafficEntry.InstallPattern = PAT_INSTALL_TRAFFIC_COLLECTOR
	trafficEntry.UninstallPattern = PAT_UNINSTALL_TRAFFIC_COLLECTOR
	trafficEntry.UpgradePattern = PAT_UPGRADE_TRAFFIC_COLLECTOR
	trafficEntry.Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameTrafficCollector)
	entries = append(entries, trafficEntry)

	podStatsCollectorVersion, podStatsCollectorInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNamePodStatsCollector)
	if podStatsCollectorVersion == "" && podStatsCollectorInstalledErr == nil {
		podStatsCollectorVersion = "6.6.6" // flag local version without tag
	}
	podStatsMsg := fmt.Sprintf("%s (Version: %s) is installed.", utils.HelmReleaseNamePodStatsCollector, podStatsCollectorVersion)
	if podStatsCollectorInstalledErr != nil {
		podStatsMsg = fmt.Sprintf("%s is not installed.\nTo gather pod/event information you need to install this component.", utils.HelmReleaseNamePodStatsCollector)
	}
	podStatsDescription := fmt.Sprintf("Collects and exposes status events of pods for services in mogenius. (Installed: %s | Available: %s)", podStatsCollectorVersion, podstatsCollectorNewestVersion)
	podEntry := punq.CreateSystemCheckEntry(utils.HelmReleaseNamePodStatsCollector, podStatsCollectorInstalledErr == nil, podStatsMsg, podStatsDescription, true, true, podStatsCollectorVersion, podstatsCollectorNewestVersion)
	podEntry.InstallPattern = PAT_INSTALL_POD_STATS_COLLECTOR
	podEntry.UninstallPattern = PAT_UNINSTALL_POD_STATS_COLLECTOR
	podEntry.UpgradePattern = PAT_UPGRADE_PODSTATS_COLLECTOR
	podEntry.Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNamePodStatsCollector)
	entries = append(entries, podEntry)

	distributionRegistryName := "distribution-registry-docker-registry"
	distriRegistryVersion, distriRegistryInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, distributionRegistryName)
	distriRegistryMsg := fmt.Sprintf("%s (Version: %s) is installed.", distributionRegistryName, distriRegistryVersion)
	if distriRegistryInstalledErr != nil {
		distriRegistryMsg = fmt.Sprintf("%s is not installed.\nTo have a private container registry running inside your cluster, you need to install this component.", distributionRegistryName)
	}
	distriDescription := "A Docker-based container registry inside Kubernetes."
	currentDistriRegistryVersion := getMostCurrentHelmChartVersion(ContainerRegistryHelmIndex, "docker-registry")
	distriEntry := punq.CreateSystemCheckEntry(NameInternalContainerRegistry, distriRegistryInstalledErr == nil, distriRegistryMsg, distriDescription, false, true, distriRegistryVersion, currentDistriRegistryVersion)
	distriEntry.InstallPattern = PAT_INSTALL_CONTAINER_REGISTRY
	distriEntry.UninstallPattern = PAT_UNINSTALL_CONTAINER_REGISTRY
	distriEntry.UpgradePattern = PAT_UPGRADE_CONTAINER_REGISTRY
	distriEntry.Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameDistributionRegistry)
	entries = append(entries, distriEntry)

	metallbVersion, metallbInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, "metallb-controller")
	metallbMsg := fmt.Sprintf("%s (Version: %s) is installed.", NameMetalLB, metallbVersion)
	if metallbInstalledErr != nil {
		metallbMsg = fmt.Sprintf("%s is not installed.\nTo have a local load balancer, you need to install this component.", NameMetalLB)
	}
	metallbDescription := "A load balancer for local clusters (e.g. Docker Desktop, k3s, minikube, etc.)."
	currentMetallbVersion := getMostCurrentHelmChartVersion(MetalLBHelmIndex, utils.HelmReleaseNameMetalLb)
	metallbEntry := punq.CreateSystemCheckEntry(NameMetalLB, metallbInstalledErr == nil, metallbMsg, metallbDescription, false, true, metallbVersion, currentMetallbVersion)
	metallbEntry.InstallPattern = PAT_INSTALL_METALLB
	metallbEntry.UninstallPattern = PAT_UNINSTALL_METALLB
	metallbEntry.UpgradePattern = PAT_UPGRADE_METALLB
	metallbEntry.Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameMetalLb)
	entries = append(entries, metallbEntry)

	keplerVersion, keplerInstalledErr := punq.IsDaemonSetInstalled(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameKepler)
	keplerMsg := fmt.Sprintf("%s (Version: %s) is installed.", NameKepler, keplerVersion)
	if keplerInstalledErr != nil {
		keplerMsg = fmt.Sprintf("%s is not installed.\nTo observe the power consumption of the cluster, you need to install this component.", NameKepler)
	}
	keplerDescription := "Kepler (Kubernetes-based Efficient Power Level Exporter) estimates workload energy/power consumption."
	currentKeplerVersion := getMostCurrentHelmChartVersion(KeplerHelmIndex, utils.HelmReleaseNameKepler)
	keplerEntry := punq.CreateSystemCheckEntry(NameKepler, keplerInstalledErr == nil, keplerMsg, keplerDescription, false, false, keplerVersion, currentKeplerVersion)
	keplerEntry.InstallPattern = PAT_INSTALL_KEPLER
	keplerEntry.UninstallPattern = PAT_UNINSTALL_KEPLER
	keplerEntry.UpgradePattern = PAT_UPGRADE_KEPLER
	keplerEntry.Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameKepler)
	entries = append(entries, keplerEntry)

	clusterIps := punq.GetClusterExternalIps(nil)
	localDevEnvMsg := "Local development environment setup complete (192.168.66.1 found)."
	contains192168661 := punqUtils.Contains(clusterIps, "192.168.66.1")
	if !contains192168661 {
		localDevEnvMsg = "Local development environment not setup. Please run 'mocli cluster local-dev-setup' to setup your local environment."
	}
	localDevSetupEntry := punq.CreateSystemCheckEntry(NameLocalDevSetup, contains192168661, localDevEnvMsg, "", false, false, "", "")
	entries = append(entries, localDevSetupEntry)

	nfsStorageClass := kubernetes.StorageClassForClusterProvider(utils.ClusterProviderCached)
	nfsStorageClassMsg := fmt.Sprintf("NFS StorageClass '%s' found.", nfsStorageClass)
	nfsStorageClassEntry := punq.CreateSystemCheckEntry(NameNfsStorageClass, nfsStorageClass != "", nfsStorageClassMsg, "", true, false, "", "")
	entries = append(entries, nfsStorageClassEntry)

	// add missing patterns
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if entry.CheckName == NameIngressController {
			entries[i].InstallPattern = PAT_INSTALL_INGRESS_CONTROLLER_TREAFIK
			entries[i].UninstallPattern = PAT_UNINSTALL_INGRESS_CONTROLLER_TREAFIK
			entries[i].UpgradePattern = PAT_UPGRADE_INGRESS_CONTROLLER_TREAFIK
			entries[i].VersionAvailable = getMostCurrentHelmChartVersion(IngressControllerTraefikHelmIndex, utils.HelmReleaseNameTraefik)
			entries[i].Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameTraefik)
		}
		if entry.CheckName == NameMetricsServer {
			entries[i].InstallPattern = PAT_INSTALL_METRICS_SERVER
			entries[i].UninstallPattern = PAT_UNINSTALL_METRICS_SERVER
			entries[i].UpgradePattern = PAT_UPGRADE_METRICS_SERVER
			entries[i].VersionAvailable = getMostCurrentHelmChartVersion(MetricsHelmIndex, utils.HelmReleaseNameMetricsServer)
			entries[i].Status = kubernetes.HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, utils.HelmReleaseNameMetricsServer)
		}
	}
	// update entries specificly for certain cluster vendors
	entries = UpdateSystemCheckStatusForClusterVendor(entries)

	return punq.GenerateSystemCheckResponse(entries)
}

func UpdateSystemCheckStatusForClusterVendor(entries []punq.SystemCheckEntry) []punq.SystemCheckEntry {
	provider, err := punq.GuessClusterProvider(nil)
	if err != nil {
		logger.Log.Errorf("UpdateSystemCheckStatusForClusterVendor Err: %s", err.Error())
		return entries
	}

	switch provider {
	case punqDtos.EKS, punqDtos.AKS, punqDtos.GKE, punqDtos.DOKS, punqDtos.OTC:
		entries = deleteSystemCheckEntryByName(entries, NameMetricsServer)
		entries = deleteSystemCheckEntryByName(entries, NameMetalLB)
		entries = deleteSystemCheckEntryByName(entries, NameLocalDevSetup)
	case punqDtos.UNKNOWN:
		logger.Log.Errorf("Unknown ClusterProvider. Not modifying anything in UpdateSystemCheckStatusForClusterVendor().")
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

func deleteSystemCheckEntryByName(entries []punq.SystemCheckEntry, name string) []punq.SystemCheckEntry {
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

func InstallAllLocalDevComponents(email string) string {
	result := ""
	result += InstallTrafficCollector() + "\n"
	result += InstallPodStatsCollector() + "\n"
	result += InstallMetricsServer() + "\n"
	result += InstallIngressControllerTreafik() + "\n"
	result += InstallCertManager() + "\n"
	result += InstallContainerRegistry() + "\n"
	result += InstallMetalLb() + "\n"
	result += InstallClusterIssuer(email, 0) + "\n"
	return result
}

func InstallTrafficCollector() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "mogenius",
		HelmRepoUrl:     MogeniusHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameTrafficCollector,
		HelmChartName:   "mogenius/" + utils.HelmReleaseNameTrafficCollector,
		HelmFlags:       fmt.Sprintf("--set global.namespace=%s --set global.stage=%s", utils.CONFIG.Kubernetes.OwnNamespace, utils.CONFIG.Misc.Stage),
		HelmTask:        structs.HelmInstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UpgradeTrafficCollector() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "mogenius",
		HelmRepoUrl:     MogeniusHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameTrafficCollector,
		HelmChartName:   "mogenius/" + utils.HelmReleaseNameTrafficCollector,
		HelmFlags:       fmt.Sprintf("--set global.namespace=%s --set global.stage=%s", utils.CONFIG.Kubernetes.OwnNamespace, utils.CONFIG.Misc.Stage),
		HelmTask:        structs.HelmUpgrade,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallPodStatsCollector() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "mogenius",
		HelmRepoUrl:     MogeniusHelmIndex,
		HelmReleaseName: utils.HelmReleaseNamePodStatsCollector,
		HelmChartName:   "mogenius/" + utils.HelmReleaseNamePodStatsCollector,
		HelmFlags:       fmt.Sprintf("--set global.namespace=%s --set global.stage=%s", utils.CONFIG.Kubernetes.OwnNamespace, utils.CONFIG.Misc.Stage),
		HelmTask:        structs.HelmInstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UpgradePodStatsCollector() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "mogenius",
		HelmRepoUrl:     MogeniusHelmIndex,
		HelmReleaseName: utils.HelmReleaseNamePodStatsCollector,
		HelmChartName:   "mogenius/" + utils.HelmReleaseNamePodStatsCollector,
		HelmFlags:       fmt.Sprintf("--set global.namespace=%s --set global.stage=%s", utils.CONFIG.Kubernetes.OwnNamespace, utils.CONFIG.Misc.Stage),
		HelmTask:        structs.HelmUpgrade,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallMetricsServer() string {
	r := ClusterHelmRequest{
		Namespace:       "default",
		HelmRepoName:    utils.HelmReleaseNameMetricsServer,
		HelmRepoUrl:     MetricsHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameMetricsServer,
		HelmChartName:   utils.HelmReleaseNameMetricsServer + "/" + utils.HelmReleaseNameMetricsServer,
		HelmFlags:       "--set args[0]='--kubelet-insecure-tls' --set args[1]='--secure-port=10250' --set args[2]='--cert-dir=/tmp' --set 'args[3]=--kubelet-preferred-address-types=InternalIP\\,ExternalIP\\,Hostname' --set args[4]='--kubelet-use-node-status-port' --set args[5]='--metric-resolution=15s'",
		HelmTask:        structs.HelmInstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UpgradeMetricsServer() string {
	r := ClusterHelmRequest{
		Namespace:       "default",
		HelmRepoName:    utils.HelmReleaseNameMetricsServer,
		HelmRepoUrl:     MetricsHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameMetricsServer,
		HelmChartName:   utils.HelmReleaseNameMetricsServer + "/" + utils.HelmReleaseNameMetricsServer,
		HelmFlags:       "--set args[0]='--kubelet-insecure-tls' --set args[1]='--secure-port=10250' --set args[2]='--cert-dir=/tmp' --set 'args[3]=--kubelet-preferred-address-types=InternalIP\\,ExternalIP\\,Hostname' --set args[4]='--kubelet-use-node-status-port' --set args[5]='--metric-resolution=15s'",
		HelmTask:        structs.HelmUpgrade,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallIngressControllerTreafik() string {
	r := ClusterHelmRequest{
		Namespace:       "default",
		HelmRepoName:    utils.HelmReleaseNameTraefik,
		HelmRepoUrl:     IngressControllerTraefikHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameTraefik,
		HelmChartName:   utils.HelmReleaseNameTraefik + "/" + utils.HelmReleaseNameTraefik,
		HelmFlags:       "",
		HelmTask:        structs.HelmInstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UpgradeIngressControllerTreafik() string {
	r := ClusterHelmRequest{
		Namespace:       "default",
		HelmRepoName:    utils.HelmReleaseNameTraefik,
		HelmRepoUrl:     IngressControllerTraefikHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameTraefik,
		HelmChartName:   utils.HelmReleaseNameTraefik + "/" + utils.HelmReleaseNameTraefik,
		HelmFlags:       "",
		HelmTask:        structs.HelmUpgrade,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallCertManager() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "jetstack",
		HelmRepoUrl:     CertManagerHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameCertManager,
		HelmChartName:   "jetstack/" + utils.HelmReleaseNameCertManager,
		HelmFlags:       fmt.Sprintf("--namespace %s --set startupapicheck.enabled=false --set installCRDs=true", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmInstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UpgradeCertManager() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "jetstack",
		HelmRepoUrl:     CertManagerHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameCertManager,
		HelmChartName:   "jetstack/" + utils.HelmReleaseNameCertManager,
		HelmFlags:       fmt.Sprintf("--namespace %s --set startupapicheck.enabled=false --set installCRDs=true", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmUpgrade,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallContainerRegistry() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "phntom",
		HelmRepoUrl:     ContainerRegistryHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameDistributionRegistry,
		HelmChartName:   "phntom/docker-registry",
		HelmFlags:       fmt.Sprintf("--namespace %s", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmInstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UpgradeContainerRegistry() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "phntom",
		HelmRepoUrl:     ContainerRegistryHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameDistributionRegistry,
		HelmChartName:   "phntom/docker-registry",
		HelmFlags:       fmt.Sprintf("--namespace %s", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmUpgrade,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallMetalLb() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    utils.HelmReleaseNameMetalLb,
		HelmRepoUrl:     MetalLBHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameMetalLb,
		HelmChartName:   utils.HelmReleaseNameMetalLb + "/" + utils.HelmReleaseNameMetalLb,
		HelmFlags:       fmt.Sprintf("--namespace %s", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmInstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		for {
			// this is important because the control plane needs some time to make the CRDs available
			time.Sleep(1 * time.Second)
			err := mokubernetes.ApplyYamlString(InstallAddressPool())
			if err != nil && !apierrors.IsAlreadyExists(err) {
				logger.Log.Errorf("Error installing metallb address pool: %s", err.Error())
			}
			if err != nil && apierrors.IsInternalError(err) {
				logger.Log.Noticef("Control plane not ready. Waiting for metallb address pool installation ...")
			}
			if err == nil {
				return
			}
		}
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})

	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UpgradeMetalLb() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    utils.HelmReleaseNameMetalLb,
		HelmRepoUrl:     MetalLBHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameMetalLb,
		HelmChartName:   utils.HelmReleaseNameMetalLb + "/" + utils.HelmReleaseNameMetalLb,
		HelmFlags:       fmt.Sprintf("--namespace %s", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmUpgrade,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallKepler() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    utils.HelmReleaseNameKepler,
		HelmRepoUrl:     KeplerHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameKepler,
		HelmChartName:   utils.HelmReleaseNameKepler + "/" + utils.HelmReleaseNameKepler,
		HelmFlags:       fmt.Sprintf(`--namespace %s --set global.namespace="%s" --set extraEnvVars.EXPOSE_IRQ_COUNTER_METRICS="false" --set extraEnvVars.EXPOSE_KUBELET_METRICS="false" --set extraEnvVars.ENABLE_PROCESS_METRICS="false"`, utils.CONFIG.Kubernetes.OwnNamespace, utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmInstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UpgradeKepler() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    utils.HelmReleaseNameKepler,
		HelmRepoUrl:     KeplerHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameKepler,
		HelmChartName:   utils.HelmReleaseNameKepler + "/" + utils.HelmReleaseNameKepler,
		HelmFlags:       fmt.Sprintf(`--namespace %s --set global.namespace="%s" --set extraEnvVars.EXPOSE_IRQ_COUNTER_METRICS="false" --set extraEnvVars.EXPOSE_KUBELET_METRICS="false" --set extraEnvVars.ENABLE_PROCESS_METRICS="false"`, utils.CONFIG.Kubernetes.OwnNamespace, utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmUpgrade,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallClusterIssuer(email string, currentRetries int) string {
	time.Sleep(3 * time.Second) // wait for cert-manager to be ready
	maxRetries := 5
	if currentRetries >= maxRetries {
		return "No suitable Ingress Controller found. Please install Traefik or Nginx Ingress Controller first."
	} else {
		fmt.Println("EEEEEEEEEEEEEE")
		ingType, err := punq.DetermineIngressControllerType(nil)
		if err != nil {
			logger.Log.Errorf("InstallClusterIssuer: Error determining ingress controller type: %s", err.Error())
		}
		fmt.Println("YYYYYYYYYYYYYYY")
		if ingType == punq.TRAEFIK || ingType == punq.NGINX {
			fmt.Println("XXXXXXXXX")
			r := ClusterHelmRequest{
				Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
				HelmRepoName:    "mogenius",
				HelmRepoUrl:     MogeniusHelmIndex,
				HelmReleaseName: utils.HelmReleaseNameClusterIssuer,
				HelmChartName:   "mogenius/mogenius-cluster-issuer",
				HelmFlags:       fmt.Sprintf(`--replace --namespace %s --set global.clusterissuermail="%s" --set global.ingressclass="%s"`, utils.CONFIG.Kubernetes.OwnNamespace, email, strings.ToLower(ingType.String())),
				HelmTask:        structs.HelmInstall,
			}
			fmt.Println("ZZZZZZZZZZZZZ")
			mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
				fmt.Println("AAAAAAAAAA")
				db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
			}, func(output string, err error) {
				fmt.Println("BBBBBBBBBBBB")
				currentRetries++
				db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
				InstallClusterIssuer(email, currentRetries)
			})
			fmt.Println("CCCCCCCCCCCCC")
			return fmt.Sprintf("Successfully triggert '%s' of '%s' (%s, %s).", r.HelmTask, r.HelmReleaseName, email, strings.ToLower(ingType.String()))
		}
		logger.Log.Noticef("No suitable Ingress Controller found (%s). Retry in 3 seconds (%d/%d) ...", ingType.String(), currentRetries, maxRetries)
		currentRetries++
		return InstallClusterIssuer(email, currentRetries)
	}
}

func UninstallTrafficCollector() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "mogenius",
		HelmRepoUrl:     MogeniusHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameTrafficCollector,
		HelmChartName:   "",
		HelmFlags:       "",
		HelmTask:        structs.HelmUninstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UninstallPodStatsCollector() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "mogenius",
		HelmRepoUrl:     MogeniusHelmIndex,
		HelmReleaseName: utils.HelmReleaseNamePodStatsCollector,
		HelmChartName:   "",
		HelmFlags:       "",
		HelmTask:        structs.HelmUninstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UninstallMetricsServer() string {
	r := ClusterHelmRequest{
		Namespace:       "default",
		HelmRepoName:    utils.HelmReleaseNameMetricsServer,
		HelmRepoUrl:     MetricsHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameMetricsServer,
		HelmChartName:   "",
		HelmFlags:       "",
		HelmTask:        structs.HelmUninstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UninstallIngressControllerTreafik() string {
	r := ClusterHelmRequest{
		Namespace:       "default",
		HelmRepoName:    utils.HelmReleaseNameTraefik,
		HelmRepoUrl:     IngressControllerTraefikHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameTraefik,
		HelmChartName:   "",
		HelmFlags:       "",
		HelmTask:        structs.HelmUninstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UninstallCertManager() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "jetstack",
		HelmRepoUrl:     CertManagerHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameCertManager,
		HelmChartName:   "",
		HelmFlags:       fmt.Sprintf("--namespace %s", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmUninstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UninstallContainerRegistry() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "phntom",
		HelmRepoUrl:     ContainerRegistryHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameDistributionRegistry,
		HelmChartName:   "",
		HelmFlags:       fmt.Sprintf("--namespace %s", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmUninstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UninstallMetalLb() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    utils.HelmReleaseNameMetalLb,
		HelmRepoUrl:     MetalLBHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameMetalLb,
		HelmChartName:   "",
		HelmFlags:       fmt.Sprintf("--namespace %s", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmUninstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UninstallKepler() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    utils.HelmReleaseNameKepler,
		HelmRepoUrl:     KeplerHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameKepler,
		HelmChartName:   "",
		HelmFlags:       fmt.Sprintf("--namespace %s", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmUninstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UninstallClusterIssuer() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "mogenius",
		HelmRepoUrl:     MogeniusHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameClusterIssuer,
		HelmChartName:   "",
		HelmFlags:       fmt.Sprintf(`--namespace %s`, utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmUninstall,
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
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

# install helm
if command -v helm >/dev/null 2>&1; then
    echo "helm is installed. Skipping installation."
else
	curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
	chmod 700 get_helm.sh
	./get_helm.sh >/dev/null
	rm get_helm.sh
	echo "helm is installed. 🚀"
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
		exit 1;
	fi
	tar -xf popeye.tar.gz popeye
	chmod +x popeye
	mv popeye /usr/local/bin/popeye
	rm popeye.tar.gz
	echo "popeye is installed. 🚀"
fi

# install grype
if command -v grype >/dev/null 2>&1; then
    echo "grype is installed. Skipping installation."
else
	curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin
	echo "grype is installed. 🚀"
fi
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

func getCurrentTrafficCollectorAndPodStatsVersion() (string, string, error) {
	data, err := utils.GetVersionData(utils.CONFIG.Misc.HelmIndex)
	if err != nil {
		return "NO_VERSION_FOUND", "NO_VERSION_FOUND", err
	}
	podstatsCollector := data.Entries[utils.HelmReleaseNamePodStatsCollector]
	podstatsResult := "NO_VERSION_FOUND"
	if len(podstatsCollector) > 0 {
		podstatsResult = podstatsCollector[0].Version
	}
	trafficCollector := data.Entries[utils.HelmReleaseNameTrafficCollector]
	trafficResult := "NO_VERSION_FOUND"
	if len(trafficCollector) > 0 {
		trafficResult = trafficCollector[0].Version
	}
	return trafficResult, podstatsResult, nil
}

func getMostCurrentHelmChartVersion(url string, chartname string) string {
	url = addIndexYAMLtoURL(url)
	data, err := utils.GetVersionData(url)
	if err != nil {
		logger.Log.Errorf("Error getting helm chart version (%s/%s): %s", url, chartname, err)
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
