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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	punqDtos "github.com/mogenius/punq/dtos"
	punq "github.com/mogenius/punq/kubernetes"
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
	NamePodStatsCollector         = "mogenius-pod-stats-collector"
	NameTrafficCollector          = "mogenius-traffic-collector"
	NameClusterIssuer             = "clusterissuer"
	NameCertManagerName           = "cert-manager"
	NameKepler                    = "Kepler"
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
		job.State = "FAILED"
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
		job.State = "FAILED"
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

var certManagerStatus punq.SystemCheckStatus = punq.UNKNOWN_STATUS
var podStatsCollectorStatus punq.SystemCheckStatus = punq.UNKNOWN_STATUS
var trafficCollectorStatus punq.SystemCheckStatus = punq.UNKNOWN_STATUS
var metricsServerStatus punq.SystemCheckStatus = punq.UNKNOWN_STATUS
var ingressCtrlStatus punq.SystemCheckStatus = punq.UNKNOWN_STATUS
var distriRegistryStatus punq.SystemCheckStatus = punq.UNKNOWN_STATUS
var metallbStatus punq.SystemCheckStatus = punq.UNKNOWN_STATUS
var clusterIssuerStatus punq.SystemCheckStatus = punq.UNKNOWN_STATUS
var keplerStatus punq.SystemCheckStatus = punq.UNKNOWN_STATUS

var keplerHostAndPort string = ""

var energyConsumptionCollectionInProgress bool = false

func EnergyConsumption() []structs.EnergyConsumptionResponse {
	if energyConsumptionCollectionInProgress {
		return structs.CurrentEnergyConsumptionResponse
	}

	if keplerHostAndPort == "" {
		keplerservice := kubernetes.ServiceWithLabels("app.kubernetes.io/component=exporter,app.kubernetes.io/name=kepler", nil)
		if keplerservice != nil {
			keplerHostAndPort = fmt.Sprintf("%s:%d", keplerservice.Spec.ClusterIP, keplerservice.Spec.Ports[0].Port)
		} else {
			logger.Log.Errorf("EnergyConsumption Err: kepler service not found.")
			return structs.CurrentEnergyConsumptionResponse
		}
		if utils.CONFIG.Misc.Stage == utils.STAGE_LOCAL {
			logger.Log.Notice("OVERWRITTEN ACTUAL IP BECAUSE RUNNING IN LOCAL MODE! 192.168.178.132:9102")
			keplerHostAndPort = "192.168.178.132:9102"
		}
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

	contextName := mokubernetes.CurrentContextName()

	dockerResult, dockerOutput, dockerErr := IsDockerInstalled()
	kubeCtlMsg := punq.StatusMessage(dockerErr, "If docker is missing in this image, we are screwed ;-)", dockerOutput)
	entries = append(entries, punq.CreateSystemCheckEntry("docker", dockerResult, kubeCtlMsg, true))

	certManagerVersion, certManagerInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, NameCertManagerName)
	certManagerMsg := fmt.Sprintf("%s (Version: %s) is installed.", NameCertManagerName, certManagerVersion)
	if certManagerInstalledErr != nil {
		certManagerMsg = fmt.Sprintf("%s is not installed in context '%s'.\nTo create ssl certificates you need to install this component.", NameCertManagerName, contextName)
	}
	certMgrEntry := punq.CreateSystemCheckEntry(NameCertManagerName, certManagerInstalledErr == nil, certManagerMsg, false)
	certMgrEntry.InstallPattern = PAT_INSTALL_CERT_MANAGER
	certMgrEntry.UninstallPattern = PAT_UNINSTALL_CERT_MANAGER
	if certManagerStatus != punq.UNKNOWN_STATUS {
		certMgrEntry.Status = certManagerStatus
	}
	entries = append(entries, certMgrEntry)

	clusterIssuerVersion, clusterIssuerInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, NameClusterIssuer)
	clusterIssuerMsg := fmt.Sprintf("%s (Version: %s) is installed.", NameClusterIssuer, clusterIssuerVersion)
	if clusterIssuerInstalledErr != nil {
		clusterIssuerMsg = fmt.Sprintf("%s is not installed in context '%s'.\nTo issue ssl certificates you need to install this component.", NameClusterIssuer, contextName)
	}
	clusterIssuerEntry := punq.CreateSystemCheckEntry(NameClusterIssuer, clusterIssuerInstalledErr == nil, clusterIssuerMsg, false)
	clusterIssuerEntry.InstallPattern = PAT_INSTALL_CLUSTER_ISSUER
	clusterIssuerEntry.UninstallPattern = PAT_UNINSTALL_CLUSTER_ISSUER
	if clusterIssuerStatus != punq.UNKNOWN_STATUS {
		clusterIssuerEntry.Status = clusterIssuerStatus
	}
	entries = append(entries, clusterIssuerEntry)

	trafficCollectorVersion, trafficCollectorInstalledErr := punq.IsDaemonSetInstalled(utils.CONFIG.Kubernetes.OwnNamespace, NameTrafficCollector)
	trafficMsg := fmt.Sprintf("%s (Version: %s) is installed.", NameTrafficCollector, trafficCollectorVersion)
	if trafficCollectorInstalledErr != nil {
		trafficMsg = fmt.Sprintf("%s is not installed in context '%s'.\nTo gather traffic information you need to install this component.", NameTrafficCollector, contextName)
	}
	trafficEntry := punq.CreateSystemCheckEntry(NameTrafficCollector, trafficCollectorInstalledErr == nil, trafficMsg, false)
	trafficEntry.InstallPattern = PAT_INSTALL_TRAFFIC_COLLECTOR
	trafficEntry.UninstallPattern = PAT_UNINSTALL_TRAFFIC_COLLECTOR
	if trafficCollectorStatus != punq.UNKNOWN_STATUS {
		trafficEntry.Status = trafficCollectorStatus
	}
	entries = append(entries, trafficEntry)

	podStatsCollectorVersion, podStatsCollectorInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, NamePodStatsCollector)
	podStatsMsg := fmt.Sprintf("%s (Version: %s) is installed.", NamePodStatsCollector, podStatsCollectorVersion)
	if podStatsCollectorInstalledErr != nil {
		podStatsMsg = fmt.Sprintf("%s is not installed in context '%s'.\nTo gather pod/event information you need to install this component.", NamePodStatsCollector, contextName)
	}
	podEntry := punq.CreateSystemCheckEntry(NamePodStatsCollector, podStatsCollectorInstalledErr == nil, podStatsMsg, true)
	podEntry.InstallPattern = PAT_INSTALL_POD_STATS_COLLECTOR
	podEntry.UninstallPattern = PAT_UNINSTALL_POD_STATS_COLLECTOR
	if podStatsCollectorStatus != punq.UNKNOWN_STATUS {
		podEntry.Status = podStatsCollectorStatus
	}
	entries = append(entries, podEntry)

	distributionRegistryName := "distribution-registry-docker-registry"
	distriRegistryVersion, distriRegistryInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, distributionRegistryName)
	distriRegistryMsg := fmt.Sprintf("%s (Version: %s) is installed.", distributionRegistryName, distriRegistryVersion)
	if distriRegistryInstalledErr != nil {
		distriRegistryMsg = fmt.Sprintf("%s is not installed in context '%s'.\nTo have a private container registry running inside your cluster, you need to install this component.", distributionRegistryName, contextName)
	}
	distriEntry := punq.CreateSystemCheckEntry(NameInternalContainerRegistry, distriRegistryInstalledErr == nil, distriRegistryMsg, false)
	distriEntry.InstallPattern = PAT_INSTALL_CONTAINER_REGISTRY
	distriEntry.UninstallPattern = PAT_UNINSTALL_CONTAINER_REGISTRY
	if distriRegistryStatus != punq.UNKNOWN_STATUS {
		distriEntry.Status = distriRegistryStatus
	}
	entries = append(entries, distriEntry)

	metallbVersion, metallbInstalledErr := punq.IsDeploymentInstalled(utils.CONFIG.Kubernetes.OwnNamespace, "metallb-controller")
	metallbMsg := fmt.Sprintf("%s (Version: %s) is installed.", NameMetalLB, metallbVersion)
	if metallbInstalledErr != nil {
		metallbMsg = fmt.Sprintf("%s is not installed in context '%s'.\nTo have a local load balancer, you need to install this component.", NameMetalLB, contextName)
	}
	metallbEntry := punq.CreateSystemCheckEntry(NameMetalLB, metallbInstalledErr == nil, metallbMsg, false)
	metallbEntry.InstallPattern = PAT_INSTALL_METALLB
	metallbEntry.UninstallPattern = PAT_UNINSTALL_METALLB
	if metallbStatus != punq.UNKNOWN_STATUS {
		metallbEntry.Status = metallbStatus
	}
	entries = append(entries, metallbEntry)

	keplerVersion, keplerInstalledErr := punq.IsDaemonSetInstalled(utils.CONFIG.Kubernetes.OwnNamespace, "kepler")
	keplerMsg := fmt.Sprintf("%s (Version: %s) is installed.", NameKepler, keplerVersion)
	if keplerInstalledErr != nil {
		keplerMsg = fmt.Sprintf("%s is not installed in context '%s'.\nTo observe the power consumption of the cluster, you need to install this component.", NameKepler, contextName)
	}
	keplerEntry := punq.CreateSystemCheckEntry(NameKepler, keplerInstalledErr == nil, keplerMsg, false)
	keplerEntry.InstallPattern = PAT_INSTALL_KEPLER
	keplerEntry.UninstallPattern = PAT_UNINSTALL_KEPLER
	if keplerStatus != punq.UNKNOWN_STATUS {
		keplerEntry.Status = keplerStatus
	}
	entries = append(entries, keplerEntry)

	clusterIps := punq.GetClusterExternalIps(nil)
	localDevEnvMsg := "Local development environment setup complete (192.168.66.1 found)."
	contains192168661 := punqUtils.Contains(clusterIps, "192.168.66.1")
	if !contains192168661 {
		localDevEnvMsg = "Local development environment not setup. Please run 'mocli cluster local-dev-setup' to setup your local environment."
	}
	localDevSetupEntry := punq.CreateSystemCheckEntry(NameLocalDevSetup, contains192168661, localDevEnvMsg, false)
	entries = append(entries, localDevSetupEntry)

	// add missing patterns
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if entry.CheckName == NameIngressController {
			entries[i].InstallPattern = PAT_INSTALL_INGRESS_CONTROLLER_TREAFIK
			entries[i].UninstallPattern = PAT_UNINSTALL_INGRESS_CONTROLLER_TREAFIK
			if ingressCtrlStatus != punq.UNKNOWN_STATUS {
				entries[i].Status = ingressCtrlStatus
			}
		}
		if entry.CheckName == NameMetricsServer {
			entries[i].InstallPattern = PAT_INSTALL_METRICS_SERVER
			entries[i].UninstallPattern = PAT_UNINSTALL_METRICS_SERVER
			if metricsServerStatus != punq.UNKNOWN_STATUS {
				entries[i].Status = metricsServerStatus
			}
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
	result += InstallClusterIssuer(email) + "\n"
	return result
}

func InstallTrafficCollector() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "mogenius",
		HelmRepoUrl:     "https://helm.mogenius.com/public",
		HelmReleaseName: "mogenius-traffic-collector",
		HelmChartName:   "mogenius/mogenius-traffic-collector",
		HelmFlags:       fmt.Sprintf("--set global.namespace=%s", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmInstall,
	}
	trafficCollectorStatus = punq.INSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		trafficCollectorStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		trafficCollectorStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallPodStatsCollector() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "mogenius",
		HelmRepoUrl:     "https://helm.mogenius.com/public",
		HelmReleaseName: "mogenius-pod-stats-collector",
		HelmChartName:   "mogenius/mogenius-pod-stats-collector",
		HelmFlags:       fmt.Sprintf("--set global.namespace=%s", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmInstall,
	}
	podStatsCollectorStatus = punq.INSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		podStatsCollectorStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		podStatsCollectorStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallMetricsServer() string {
	//helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/
	//helm upgrade --install metrics-server metrics-server/metrics-server

	r := ClusterHelmRequest{
		Namespace:       "default",
		HelmRepoName:    "metrics-server",
		HelmRepoUrl:     "https://kubernetes-sigs.github.io/metrics-server/",
		HelmReleaseName: "metrics-server",
		HelmChartName:   "metrics-server/metrics-server",
		HelmFlags:       "--set args[0]='--kubelet-insecure-tls' --set args[1]='--secure-port=10250' --set args[2]='--cert-dir=/tmp' --set 'args[3]=--kubelet-preferred-address-types=InternalIP\\,ExternalIP\\,Hostname' --set args[4]='--kubelet-use-node-status-port' --set args[5]='--metric-resolution=15s'",
		HelmTask:        structs.HelmInstall,
	}
	metricsServerStatus = punq.INSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		metricsServerStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		metricsServerStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallIngressControllerTreafik() string {
	// helm repo add traefik https://traefik.github.io/charts
	// helm install traefik traefik/traefik

	r := ClusterHelmRequest{
		Namespace:       "default",
		HelmRepoName:    "traefik",
		HelmRepoUrl:     "https://traefik.github.io/charts",
		HelmReleaseName: "traefik",
		HelmChartName:   "traefik/traefik",
		HelmFlags:       "",
		HelmTask:        structs.HelmInstall,
	}
	ingressCtrlStatus = punq.INSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		ingressCtrlStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		ingressCtrlStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallCertManager() string {
	// helm repo add jetstack https://charts.jetstack.io
	// helm install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace

	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "jetstack",
		HelmRepoUrl:     "https://charts.jetstack.io",
		HelmReleaseName: "cert-manager",
		HelmChartName:   "jetstack/cert-manager",
		HelmFlags:       fmt.Sprintf("--namespace %s --set startupapicheck.enabled=false --set installCRDs=true", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmInstall,
	}
	certManagerStatus = punq.INSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		certManagerStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		certManagerStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallContainerRegistry() string {
	// helm repo add phntom https://phntom.kix.co.il/charts/
	// helm install distribution-registry phntom/docker-registry -n mogenius

	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "phntom",
		HelmRepoUrl:     "https://phntom.kix.co.il/charts/",
		HelmReleaseName: "distribution-registry",
		HelmChartName:   "phntom/docker-registry",
		HelmFlags:       fmt.Sprintf("--namespace %s", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmInstall,
	}
	distriRegistryStatus = punq.INSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		distriRegistryStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		distriRegistryStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallMetalLb() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "metallb",
		HelmRepoUrl:     "https://metallb.github.io/metallb",
		HelmReleaseName: "metallb",
		HelmChartName:   "metallb/metallb",
		HelmFlags:       fmt.Sprintf("--namespace %s", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmInstall,
	}
	metallbStatus = punq.INSTALLING

	installAfterHelmChart := func() {
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
	}
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, installAfterHelmChart, nil)

	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallKepler() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "kepler",
		HelmRepoUrl:     "https://sustainable-computing-io.github.io/kepler-helm-chart",
		HelmReleaseName: "kepler",
		HelmChartName:   "kepler/kepler",
		HelmFlags:       fmt.Sprintf(`--namespace %s --set global.namespace="%s" --set extraEnvVars.EXPOSE_IRQ_COUNTER_METRICS="false" --set extraEnvVars.EXPOSE_KUBELET_METRICS="false" --set extraEnvVars.ENABLE_PROCESS_METRICS="false"`, utils.CONFIG.Kubernetes.OwnNamespace, utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmInstall,
	}
	keplerStatus = punq.INSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		keplerStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		keplerStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallClusterIssuer(email string) string {
	// helm install clusterissuer mogenius/mogenius-cluster-issuer --set global.clusterissuermail="ruediger@mogenius.com" --set global.ingressclass="traefik"

	ingType, err := punq.DetermineIngressControllerType(nil)
	if err != nil {
		return fmt.Sprintf("Error determining ingress controller type: %s", err.Error())
	}

	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "mogenius",
		HelmRepoUrl:     "https://helm.mogenius.com/public",
		HelmReleaseName: "clusterissuer",
		HelmChartName:   "mogenius/mogenius-cluster-issuer",
		HelmFlags:       fmt.Sprintf(`--namespace %s --set global.clusterissuermail="%s" --set global.ingressclass="%s"`, utils.CONFIG.Kubernetes.OwnNamespace, email, strings.ToLower(ingType.String())),
		HelmTask:        structs.HelmInstall,
	}
	clusterIssuerStatus = punq.INSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		clusterIssuerStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		clusterIssuerStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s' (%s, %s).", r.HelmTask, r.HelmReleaseName, email, strings.ToLower(ingType.String()))
}

func UninstallTrafficCollector() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "mogenius",
		HelmRepoUrl:     "https://helm.mogenius.com/public",
		HelmReleaseName: "mogenius-traffic-collector",
		HelmChartName:   "",
		HelmFlags:       "",
		HelmTask:        structs.HelmUninstall,
	}
	trafficCollectorStatus = punq.UNINSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		trafficCollectorStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		trafficCollectorStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UninstallPodStatsCollector() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "mogenius",
		HelmRepoUrl:     "https://helm.mogenius.com/public",
		HelmReleaseName: "mogenius-pod-stats-collector",
		HelmChartName:   "",
		HelmFlags:       "",
		HelmTask:        structs.HelmUninstall,
	}
	podStatsCollectorStatus = punq.UNINSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		podStatsCollectorStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		podStatsCollectorStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UninstallMetricsServer() string {
	r := ClusterHelmRequest{
		Namespace:       "default",
		HelmRepoName:    "metrics-server",
		HelmRepoUrl:     "https://kubernetes-sigs.github.io/metrics-server/",
		HelmReleaseName: "metrics-server",
		HelmChartName:   "",
		HelmFlags:       "",
		HelmTask:        structs.HelmUninstall,
	}
	metricsServerStatus = punq.UNINSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		metricsServerStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		metricsServerStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UninstallIngressControllerTreafik() string {
	r := ClusterHelmRequest{
		Namespace:       "default",
		HelmRepoName:    "traefik",
		HelmRepoUrl:     "https://traefik.github.io/charts",
		HelmReleaseName: "traefik",
		HelmChartName:   "",
		HelmFlags:       "",
		HelmTask:        structs.HelmUninstall,
	}
	ingressCtrlStatus = punq.UNINSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		ingressCtrlStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		ingressCtrlStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UninstallCertManager() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "jetstack",
		HelmRepoUrl:     "https://charts.jetstack.io",
		HelmReleaseName: "cert-manager",
		HelmChartName:   "",
		HelmFlags:       fmt.Sprintf("--namespace %s", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmUninstall,
	}
	certManagerStatus = punq.UNINSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		certManagerStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		certManagerStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UninstallContainerRegistry() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "phntom",
		HelmRepoUrl:     "https://phntom.kix.co.il/charts/",
		HelmReleaseName: "distribution-registry",
		HelmChartName:   "",
		HelmFlags:       fmt.Sprintf("--namespace %s", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmUninstall,
	}
	distriRegistryStatus = punq.UNINSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		distriRegistryStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		distriRegistryStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UninstallMetalLb() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "metallb",
		HelmRepoUrl:     "https://metallb.github.io/metallb",
		HelmReleaseName: "metallb",
		HelmChartName:   "",
		HelmFlags:       fmt.Sprintf("--namespace %s", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmUninstall,
	}
	metallbStatus = punq.UNINSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		metallbStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		metallbStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UninstallKepler() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "kepler",
		HelmRepoUrl:     "https://sustainable-computing-io.github.io/kepler-helm-chart",
		HelmReleaseName: "kepler",
		HelmChartName:   "",
		HelmFlags:       fmt.Sprintf("--namespace %s", utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmUninstall,
	}
	keplerStatus = punq.UNINSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		keplerStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		keplerStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func UninstallClusterIssuer() string {
	r := ClusterHelmRequest{
		Namespace:       utils.CONFIG.Kubernetes.OwnNamespace,
		HelmRepoName:    "mogenius",
		HelmRepoUrl:     "https://helm.mogenius.com/public",
		HelmReleaseName: "clusterissuer",
		HelmChartName:   "",
		HelmFlags:       fmt.Sprintf(`--namespace %s`, utils.CONFIG.Kubernetes.OwnNamespace),
		HelmTask:        structs.HelmUninstall,
	}
	clusterIssuerStatus = punq.INSTALLING
	mokubernetes.CreateHelmChartCmd(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, r.HelmFlags, func() {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' succeded.", r.HelmTask, r.HelmReleaseName), structs.Installation, structs.Info)
		clusterIssuerStatus = punq.UNKNOWN_STATUS
	}, func(output string, err error) {
		db.AddLogToDb(r.HelmReleaseName, fmt.Sprintf("'%s' of '%s' FAILED with Reason: %s", r.HelmTask, r.HelmReleaseName, output), structs.Installation, structs.Error)
		clusterIssuerStatus = punq.UNKNOWN_STATUS
	})
	return fmt.Sprintf("Successfully triggert '%s' of '%s'.", r.HelmTask, r.HelmReleaseName)
}

func InstallDefaultApplications() string {
	defaultAppsConfigmap := punq.ConfigMapFor(utils.CONFIG.Kubernetes.OwnNamespace, "mogenius-k8s-manager-default-apps", false, nil)
	if defaultAppsConfigmap != nil {
		if installCommands, exists := defaultAppsConfigmap.Data["install-commands"]; exists {
			return installCommands
		}
	}
	return ""
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
