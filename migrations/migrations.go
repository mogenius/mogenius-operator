package migrations

import (
	"fmt"
	"mogenius-k8s-manager/db"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"strings"

	punq "github.com/mogenius/punq/kubernetes"
)

func ExecuteMigrations() {
	name, err := _PvcMigration1()
	if err != nil {
		logger.Log.Infof("Migration ('%s'): %s", name, err.Error())
	}
}

func _PvcMigration1() (string, error) {
	migrationName := utils.GetFunctionName()
	if db.IsMigrationAlreadyApplied(migrationName) {
		return migrationName, fmt.Errorf("Migration already applied.")
	}

	pvcs := punq.AllPersistentVolumeClaims("", nil)
	for _, pvc := range pvcs {
		if strings.HasPrefix(pvc.Name, utils.CONFIG.Misc.NfsPodPrefix) {
			volumeName := strings.Replace(pvc.Name, fmt.Sprintf("%s-", utils.CONFIG.Misc.NfsPodPrefix), "", 1)
			pvc.Labels = kubernetes.MoAddLabels(&pvc.Labels, map[string]string{
				"mo-nfs-volume-identifier": pvc.Name,
				"mo-nfs-volume-name":       volumeName,
			})
			punq.UpdateK8sPersistentVolumeClaim(pvc, nil)
			// now also update auto-created PVC
			connectedPvc, err := punq.GetPersistentVolumeClaim(pvc.Namespace, volumeName, nil)
			if err == nil && connectedPvc != nil {
				connectedPvc.Labels = kubernetes.MoAddLabels(&connectedPvc.Labels, map[string]string{
					"mo-nfs-volume-identifier": pvc.Name,
					"mo-nfs-volume-name":       volumeName,
				})
				punq.UpdateK8sPersistentVolumeClaim(*connectedPvc, nil)
			}

			logger.Log.Info("Updated PVC: ", pvc.Name)
		}
	}
	pvs := punq.AllPersistentVolumesRaw("", nil)
	for _, pv := range pvs {
		if strings.HasPrefix(pv.Spec.ClaimRef.Name, utils.CONFIG.Misc.NfsPodPrefix) {
			volumeName := strings.Replace(pv.Spec.ClaimRef.Name, fmt.Sprintf("%s-", utils.CONFIG.Misc.NfsPodPrefix), "", 1)
			pv.Labels = kubernetes.MoAddLabels(&pv.Labels, map[string]string{
				"mo-nfs-volume-identifier": pv.Spec.ClaimRef.Name,
				"mo-nfs-volume-name":       volumeName,
			})
			punq.UpdateK8sPersistentVolume(pv, nil)
			logger.Log.Info("Updated PV: ", pv.Name)
		}
	}

	logger.Log.Infof("Migration '%s' applied successfuly.", migrationName)
	db.AddMigrationToDb(migrationName)
	return migrationName, nil
}
