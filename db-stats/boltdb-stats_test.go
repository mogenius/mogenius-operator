package dbstats

import (
	"fmt"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"testing"
	"time"

	"math/rand/v2"

	bolt "go.etcd.io/bbolt"
)

// test the functionality of the custom resource with a basic pod
func TestAddInterfaceStatsToDbCreateDBs(t *testing.T) {
	stat := generateRandomInterfaceStats()

	utils.CONFIG.Kubernetes.BboltDbStatsPath = "/tmp/test01.db"
	utils.CONFIG.Stats.MaxDataPoints = 1000

	getControllerFunc = func(namespace string, podName string) *kubernetes.K8sController {
		return &kubernetes.K8sController{
			Kind:      "Deployment",
			Name:      podName,
			Namespace: namespace,
		}
	}
	Start()

	tx, err := dbStats.Begin(false)
	if err != nil {
		t.Errorf("Error beginning transaction: %v", err)
	}

	// check if db has a bucket for the namespace
	if !bucketExists(tx, stat.Namespace) {
		dbStatsLogger.Info("Bucket for namespace does not exist and should be created once the stat is added", "namespace", stat.Namespace)
	}
	err = tx.Rollback()
	if err != nil {
		t.Error(err)
	}
	AddInterfaceStatsToDb(stat)

	tx, err = dbStats.Begin(false)
	if err != nil {
		t.Errorf("Error beginning transaction: %v", err)
	}
	if !bucketExists(tx, stat.Namespace) {
		t.Errorf("Bucket for namespace %s does not exist but should have been created!", stat.Namespace)
	}
	err = tx.Rollback()
	if err != nil {
		t.Error(err)
	}
}

func TestAddInterfaceStatsToDbLimitDataPoints(t *testing.T) {
	utils.CONFIG.Kubernetes.BboltDbStatsPath = "/tmp/test02.db"
	utils.CONFIG.Stats.MaxDataPoints = 3
	Start()

	getControllerFunc = func(namespace string, podName string) *kubernetes.K8sController {
		_ = podName
		return &kubernetes.K8sController{
			Kind:      "Deployment",
			Name:      "TESTCONTROLLER",
			Namespace: namespace,
		}
	}
	// add 3 random interface stats for TESTCONTROLLER
	for i := 0; i < 20; i++ {
		stats := generateRandomInterfaceStats()
		stats.Namespace = "TESTNS"
		AddInterfaceStatsToDb(stats)
	}

	tx, err := dbStats.Begin(false)
	if err != nil {
		t.Errorf("Error beginning transaction: %v", err)
	}
	defer func() {
		err := tx.Rollback()
		if err != nil {
			t.Error(err)
		}
	}()

	//check if the data points are limited to 3
	bucket := getNestedBucket(tx, []string{"TESTNS", "TESTCONTROLLER"})
	if bucket == nil {
		t.Errorf("Bucket for namespace TESTCONTROLLER does not exist but should have been created!") //TODO subbucket should exist but bolt 'forgets' it
	}

	if bucket.Stats().KeyN != utils.CONFIG.Stats.MaxDataPoints+1 {
		t.Errorf("Expected %d data points but got %d", utils.CONFIG.Stats.MaxDataPoints, bucket.Stats().KeyN)
	}

}

func getNestedBucket(tx *bolt.Tx, bucketChain []string) *bolt.Bucket {

	mainBucket := tx.Bucket([]byte(TRAFFIC_BUCKET_NAME))
	if mainBucket == nil {
		return nil
	}
	result := mainBucket
	for _, v := range bucketChain {
		result = result.Bucket([]byte(v))
		if result == nil {
			return nil
		}
	}
	return result
}

func bucketExists(tx *bolt.Tx, bucketName string) bool {
	bucket := getNestedBucket(tx, []string{bucketName})
	return bucket != nil
}

func generateRandomInterfaceStats() structs.InterfaceStats {
	return structs.InterfaceStats{
		Ip:                 fmt.Sprintf("192.168.%d.%d", rand.IntN(255), rand.IntN(255)),
		PodName:            fmt.Sprintf("pod-%d", rand.IntN(1000)),
		Namespace:          fmt.Sprintf("namespace-%d", rand.IntN(100)),
		PacketsSum:         rand.Uint64(),
		TransmitBytes:      rand.Uint64(),
		ReceivedBytes:      rand.Uint64(),
		UnknownBytes:       rand.Uint64(),
		LocalTransmitBytes: rand.Uint64(),
		LocalReceivedBytes: rand.Uint64(),
		TransmitStartBytes: rand.Uint64(),
		ReceivedStartBytes: rand.Uint64(),
		StartTime:          time.Now().Add(-time.Duration(rand.IntN(1000)) * time.Hour).Format(time.RFC3339),
		CreatedAt:          time.Now().Format(time.RFC3339),
		SocketConnections: map[string]uint64{
			"conn1": rand.Uint64(),
			"conn2": rand.Uint64(),
		},
	}
}
