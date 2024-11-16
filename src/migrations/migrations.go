package migrations

import (
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/db"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/utils"
	"strings"
)

var migrationLogger *slog.Logger

func Setup(logManagerModule interfaces.LogManagerModule) {
	migrationLogger = logManagerModule.CreateLogger("migrations")
}

func ExecuteMigrations() {
	name, err := _PvcMigration1()
	if err != nil {
		migrationLogger.Info("Migration", "name", name, "error", err)
	}
}

func _PvcMigration1() (string, error) {
	migrationName := utils.GetFunctionName()
	if db.IsMigrationAlreadyApplied(migrationName) {
		return migrationName, fmt.Errorf("Migration already applied.")
	}

	pvcs := kubernetes.AllPersistentVolumeClaims("")
	for _, pvc := range pvcs {
		if strings.HasPrefix(pvc.Name, utils.NFS_POD_PREFIX) {
			volumeName := strings.Replace(pvc.Name, fmt.Sprintf("%s-", utils.NFS_POD_PREFIX), "", 1)
			pvc.Labels = kubernetes.MoAddLabels(&pvc.Labels, map[string]string{
				kubernetes.LabelKeyVolumeIdentifier: pvc.Name,
				kubernetes.LabelKeyVolumeName:       volumeName,
			})
			kubernetes.UpdateK8sPersistentVolumeClaim(pvc)
			// now also update auto-created PVC
			connectedPvc, err := kubernetes.GetPersistentVolumeClaim(pvc.Namespace, volumeName)
			if err == nil && connectedPvc != nil {
				connectedPvc.Labels = kubernetes.MoAddLabels(&connectedPvc.Labels, map[string]string{
					kubernetes.LabelKeyVolumeIdentifier: pvc.Name,
					kubernetes.LabelKeyVolumeName:       volumeName,
				})
				kubernetes.UpdateK8sPersistentVolumeClaim(*connectedPvc)
			}

			migrationLogger.Info("Updated PVC", "name", pvc.Name)
		}
	}
	pvs := kubernetes.AllPersistentVolumesRaw()
	for _, pv := range pvs {
		if pv.Spec.ClaimRef != nil {
			if strings.HasPrefix(pv.Spec.ClaimRef.Name, utils.NFS_POD_PREFIX) {
				pv.Labels = kubernetes.MoAddLabels(&pv.Labels, map[string]string{
					kubernetes.LabelKeyVolumeIdentifier: pv.Spec.ClaimRef.Name,
					kubernetes.LabelKeyVolumeName: strings.Replace(
						pv.Spec.ClaimRef.Name,
						fmt.Sprintf("%s-", utils.NFS_POD_PREFIX),
						"",
						1,
					),
				})
				kubernetes.UpdateK8sPersistentVolume(pv)
				migrationLogger.Info("Updated PV", "name", pv.Name)
			}
		}
	}

	migrationLogger.Info("Migration applied successfuly.", "migrationName", migrationName)
	err := db.AddMigrationToDb(migrationName)
	if err != nil {
		return migrationName, fmt.Errorf("Migration '%s' applied successfuly, but could not be added to migrations table: %s", migrationName, err.Error())
	}
	return migrationName, nil
}
