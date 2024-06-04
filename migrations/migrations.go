package migrations

import (
	"fmt"
	"mogenius-k8s-manager/db"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/utils"
	"strings"

	punq "github.com/mogenius/punq/kubernetes"
	log "github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ExecuteMigrations() {
	name, err := _PvcMigration1()
	if err != nil {
		log.Infof("Migration ('%s'): %s", name, err.Error())
	}

	name, err = _PvMigration2()
	if err != nil {
		log.Infof("Migration ('%s'): %s", name, err.Error())
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
				kubernetes.LabelKeyVolumeIdentifier: pvc.Name,
				kubernetes.LabelKeyVolumeName:       volumeName,
			})
			punq.UpdateK8sPersistentVolumeClaim(pvc, nil)
			// now also update auto-created PVC
			connectedPvc, err := punq.GetPersistentVolumeClaim(pvc.Namespace, volumeName, nil)
			if err == nil && connectedPvc != nil {
				connectedPvc.Labels = kubernetes.MoAddLabels(&connectedPvc.Labels, map[string]string{
					kubernetes.LabelKeyVolumeIdentifier: pvc.Name,
					kubernetes.LabelKeyVolumeName:       volumeName,
				})
				punq.UpdateK8sPersistentVolumeClaim(*connectedPvc, nil)
			}

			log.Info("Updated PVC: ", pvc.Name)
		}
	}
	pvs := punq.AllPersistentVolumesRaw(nil)
	for _, pv := range pvs {
		if pv.Spec.ClaimRef != nil {
			if strings.HasPrefix(pv.Spec.ClaimRef.Name, utils.CONFIG.Misc.NfsPodPrefix) {
				volumeName := strings.Replace(pv.Spec.ClaimRef.Name, fmt.Sprintf("%s-", utils.CONFIG.Misc.NfsPodPrefix), "", 1)
				pv.Labels = kubernetes.MoAddLabels(&pv.Labels, map[string]string{
					kubernetes.LabelKeyVolumeIdentifier: pv.Spec.ClaimRef.Name,
					kubernetes.LabelKeyVolumeName:       volumeName,
				})
				punq.UpdateK8sPersistentVolume(pv, nil)
				log.Info("Updated PV: ", pv.Name)
			}
		}
	}

	log.Infof("Migration '%s' applied successfuly.", migrationName)
	err := db.AddMigrationToDb(migrationName)
	if err != nil {
		return migrationName, fmt.Errorf("Migration '%s' applied successfuly, but could not be added to migrations table: %s", migrationName, err.Error())
	}
	return migrationName, nil
}

func _PvMigration2() (string, error) {
	migrationName := utils.GetFunctionName()
	if db.IsMigrationAlreadyApplied(migrationName) {
		return migrationName, fmt.Errorf("Migration already applied.")
	}

	selector := metav1.ListOptions{
		LabelSelector: kubernetes.LabelKeyVolumeName,
	}

	pvs := kubernetes.PersistentVolumes(&selector, nil)
	for _, pv := range pvs {
		if !kubernetes.ContainsString(pv.ObjectMeta.Finalizers, kubernetes.FinalizerName) {
			// Add finalizer
			pv.ObjectMeta.Finalizers = append(pv.ObjectMeta.Finalizers, kubernetes.FinalizerName)
			punq.UpdateK8sPersistentVolume(pv, nil)
			log.Info("Updated PV: ", pv.Name)
		}
	}

	log.Infof("Migration '%s' applied successfuly.", migrationName)
	err := db.AddMigrationToDb(migrationName)
	if err != nil {
		return migrationName, fmt.Errorf("Migration '%s' applied successfuly, but could not be added to migrations table: %s", migrationName, err.Error())
	}
	return migrationName, nil
}
