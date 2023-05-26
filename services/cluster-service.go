package services

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/shirou/gopsutil/v3/disk"
)

const BUCKETNAME = "mogenius-backup"
const DEBUG_AWS_ACCESS_KEY_ID = "ASIAZNXZOUKFCEK3TPOL"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         // TEMP Credentials. Not security relevant
const DEBUG_AWS_SECRET_KEY = "xTsv35O30o87m6DuWOscHpKbxbXJeo0vS9iFkGwY"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        // TEMP Credentials. Not security relevant
const DEBUG_AWS_TOKEN = "IQoJb3JpZ2luX2VjELz//////////wEaDGV1LWNlbnRyYWwtMSJHMEUCIQCdZPuNWJCNJOSbMBhRtQb8W/0ylEV/ge1fiWFWgmD8ywIgEy/X2IAopx69LIGQQS+c2pRo4cSRFSslylRs8J7eUawq3AEIpf//////////ARABGgw2NDc5ODk0Njk4MzQiDPCjbP1jO5NAL96r2yqwAa3cCaeF8s1x2Zs8vAU+gRfK/tUZac8XjnJsjIxbmikiDPLuyPonsymuAd9D4ISK4fLeUU+BUU899fLHjIa2bWXRx1OrmGPIK3d/qBZF3pUPRid5AV8IRMiiP2sMI5RZzKpJfuWHH5WLknw0P7HYvusUlgAR4AgqPabHAE0c2Q1qaplJQrBXGeXCtMzs386OSPBQGogeBGn9Eu/l8QpySaA6RE3KgwvRELcvacMtmcdxMJ2H1aEGOpgBFNijTvMHK7D1pSOmvDfx9p9wSHZT/Red/G1CFWjUtV2H9+H4N+qrZTX2A4I9UGVEc+UlAQlIOAXPli2WTSPOdB7txbKsozU1YPVbi/gSZFXmGy8EFJml3bkg4HqSlozHLB/f1Ib81n9eWoUPOXp5SMwn6izW4ZZB3g8QSV6btOx2+s+Pm4BsLHICMhg3Rr0KI6ThnNhXcj8=" // TEMP Credentials. Not security relevant

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
	var wg sync.WaitGroup

	job := structs.CreateJob("Install Helm Chart "+r.HelmReleaseName, r.NamespaceId, nil, nil)
	job.Start()
	job.AddCmds(mokubernetes.ExecuteHelmChartTask(&job, r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, &wg))
	wg.Wait()
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

func InstallMogeniusNfsStorage(r NfsStorageInstallRequest) interface{} {
	nfsStatus := mokubernetes.CheckIfMogeniusNfsIsRunning()
	if !nfsStatus.IsInstalled {
		var wg sync.WaitGroup
		job := structs.CreateJob("Install mogenius nfs-storage.", "", nil, nil)
		job.Start()
		job.AddCmds(mokubernetes.InstallMogeniusNfsStorage(&job, r.ClusterProvider, &wg))
		wg.Wait()
		job.Finish()
		return job
	} else {
		nfsStatus.Error = "Mogenius NFS storage has already been installed."
		return nfsStatus
	}
}

func UninstallMogeniusNfsStorage(r NfsStorageInstallRequest) interface{} {
	var wg sync.WaitGroup
	job := structs.CreateJob("Uninstall mogenius nfs-storage.", "", nil, nil)
	job.Start()
	job.AddCmds(mokubernetes.UninstallMogeniusNfsStorage(&job, &wg))
	wg.Wait()
	job.Finish()
	return job
}

func CreateMogeniusNfsVolume(r NfsVolumeRequest) structs.DefaultResponse {
	nfsStatus := mokubernetes.CheckIfMogeniusNfsIsRunning()
	if nfsStatus.IsInstalled {
		var wg sync.WaitGroup
		job := structs.CreateJob("Create mogenius nfs-volume.", r.NamespaceId, nil, nil)
		job.Start()
		job.AddCmd(mokubernetes.CreateMogeniusNfsPersistentVolumeClaim(&job, r.NamespaceName, r.VolumeName, r.SizeInGb, &wg))
		wg.Wait()
		job.Finish()
		return job.DefaultReponse()
	} else {
		result := structs.DefaultResponse{}
		result.Error = "Mogenius NFS storage has NOT been installed."
		result.Success = false
		return result
	}
}

func DeleteMogeniusNfsVolume(r NfsVolumeRequest) structs.DefaultResponse {
	nfsStatus := mokubernetes.CheckIfMogeniusNfsIsRunning()
	if nfsStatus.IsInstalled {
		var wg sync.WaitGroup
		job := structs.CreateJob("Delete mogenius nfs-volume.", r.NamespaceId, nil, nil)
		job.Start()
		job.AddCmd(mokubernetes.DeleteMogeniusNfsPersistentVolumeClaim(&job, r.NamespaceName, r.VolumeName, &wg))
		wg.Wait()
		job.Finish()
		// update mogenius-k8s-manager volume mounts
		mokubernetes.UpdateK8sManagerVolumeMounts(r.VolumeName, r.NamespaceName)
		return job.DefaultReponse()
	} else {
		result := structs.DefaultResponse{}
		result.Error = "Mogenius NFS storage has NOT been installed."
		result.Success = false
		return result
	}
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
	logger.Log.Infof("💾: '%s' -> %s / %s (%s)", mountPath, utils.BytesToHumanReadable(int64(result.UsedBytes)), utils.BytesToHumanReadable(int64(result.TotalBytes)), fmt.Sprintf("%.1f%%", usage.UsedPercent))
	return result
}

func StatsMogeniusNfsNamespace(r NfsNamespaceStatsRequest) []NfsVolumeStatsResponse {
	result := []NfsVolumeStatsResponse{}

	// get all pvc for single namespace
	pvcs := mokubernetes.AllPersistentVolumeClaims(r.NamespaceName)

	for _, pvc := range pvcs {
		entry := NfsVolumeStatsResponse{
			VolumeName: pvc.Name,
			FreeBytes:  0,
			UsedBytes:  0,
			TotalBytes: 0,
		}

		mountPath := utils.MountPath(r.NamespaceName, pvc.Name, "/")
		usage, err := disk.Usage(mountPath)
		if err != nil {
			logger.Log.Errorf("StatsMogeniusNfsVolume Err: %s %s", mountPath, err.Error())
			continue
		} else {
			entry.FreeBytes = usage.Free
			entry.UsedBytes = usage.Used
			entry.TotalBytes = usage.Total
		}
		logger.Log.Infof("💾: '%s' -> %s / %s (%s)", mountPath, utils.BytesToHumanReadable(int64(entry.UsedBytes)), utils.BytesToHumanReadable(int64(entry.TotalBytes)), fmt.Sprintf("%.1f%%", usage.UsedPercent))
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

	msg := fmt.Sprintf("Successfully restored volume (%s) from S3!\n", utils.BytesToHumanReadable(downloadedBytes))
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

		fileBytes, err := ioutil.ReadFile(path)
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

	logger.Log.Infof("Successfully uploaded zip file (%s) to S3! -> %s\n", utils.BytesToHumanReadable(result.Bytes), result.DownloadUrl)

	return result
}

type K8sManagerUpgradeRequest struct {
	Command string `json:"command"` // complete helm command from platform ui
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
	Namespace       string `json:"namespace"`
	NamespaceId     string `json:"namespaceId"`
	HelmRepoName    string `json:"helmRepoName"`
	HelmRepoUrl     string `json:"helmRepoUrl"`
	HelmReleaseName string `json:"helmReleaseName"`
	HelmChartName   string `json:"HelmChartName"`
	HelmTask        string `json:"helmTask"` // install, upgrade, uninstall
}

func ClusterHelmRequestExample() ClusterHelmRequest {
	return ClusterHelmRequest{
		Namespace:       "mogenius",
		NamespaceId:     "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		HelmRepoName:    "bitnami",
		HelmRepoUrl:     "https://charts.bitnami.com/bitnami",
		HelmReleaseName: "test-helm-release",
		HelmChartName:   "bitnami/nginx",
		HelmTask:        "install",
	}
}

type ClusterHelmUninstallRequest struct {
	NamespaceId     string `json:"namespaceId"`
	HelmReleaseName string `json:"helmReleaseName"`
}

func ClusterHelmUninstallRequestExample() ClusterHelmUninstallRequest {
	return ClusterHelmUninstallRequest{
		NamespaceId:     "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		HelmReleaseName: "test-helm-release",
	}
}

type NfsStorageInstallRequest struct {
	ClusterProvider string `json:"ClusterProvider"` // "BRING_YOUR_OWN", "EKS", "AKS", "GKE", "DOCKER_ENTERPRISE", "DOKS", "LINODE", "IBM", "ACK", "OKE", "OTC", "OPEN_SHIFT"
}

func NfsStorageInstallRequestExample() NfsStorageInstallRequest {
	return NfsStorageInstallRequest{
		ClusterProvider: "AKS",
	}
}

type NfsVolumeRequest struct {
	NamespaceId   string `json:"namespaceId"`
	NamespaceName string `json:"namespaceName"`
	VolumeName    string `json:"volumeName"`
	SizeInGb      int    `json:"sizeInGb"`
}

func NfsVolumeRequestExample() NfsVolumeRequest {
	return NfsVolumeRequest{
		NamespaceId:   "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		NamespaceName: "bene-test",
		VolumeName:    "my-fancy-volume-name",
		SizeInGb:      10,
	}
}

type NfsVolumeStatsRequest struct {
	NamespaceName string `json:"namespaceName"`
	VolumeName    string `json:"volumeName"`
}

func NfsVolumeStatsRequestExample() NfsVolumeStatsRequest {
	return NfsVolumeStatsRequest{
		NamespaceName: "bene-test",
		VolumeName:    "my-fancy-volume-name",
	}
}

type NfsNamespaceStatsRequest struct {
	NamespaceName string `json:"namespaceName"`
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
	NamespaceId        string `json:"namespaceId"`
	NamespaceName      string `json:"namespaceName"`
	VolumeName         string `json:"volumeName"`
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
	NamespaceId        string `json:"namespaceId"`
	NamespaceName      string `json:"namespaceName"`
	VolumeName         string `json:"volumeName"`
	BackupKey          string `json:"backupKey"`
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
