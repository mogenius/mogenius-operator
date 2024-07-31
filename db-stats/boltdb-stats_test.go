package dbstats

import (
	"fmt"
	"log"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
	"golang.org/x/exp/rand"
)

// test the functionality of the custom resource with a basic pod
func TestAddInterfaceStatsToDbCreateDBs(t *testing.T) {
	stat := generateRandomInterfaceStats()

	utils.CONFIG.Kubernetes.BboltDbStatsPath = "/tmp/test.db"
	utils.CONFIG.Stats.MaxDataPoints = 1000

	getControllerFunc = func(namespace string, podName string) *kubernetes.K8sController {
		return &kubernetes.K8sController{
			Kind:      "Deployment",
			Name:      stat.PodName,
			Namespace: stat.Namespace,
		}
	}
	Init()

	// check if db has a bucket for the namespace
	if !bucketExists(dbStats, stat.Namespace) {
		t.Logf("Bucket for namespace %s does not exist and should be created once the stat is added", stat.Namespace)
	}
	AddInterfaceStatsToDb(stat)

	if !bucketExists(dbStats, stat.Namespace) {
		t.Errorf("Bucket for namespace %s does not exist but should have been created!", stat.Namespace)
	}
}

func TestAddInterfaceStatsToDbLimitDataPoints(t *testing.T) {
	utils.CONFIG.Kubernetes.BboltDbStatsPath = "/tmp/test.db"
	utils.CONFIG.Stats.MaxDataPoints = 3
	Init()

	getControllerFunc = func(namespace string, podName string) *kubernetes.K8sController {
		return &kubernetes.K8sController{
			Kind:      "Deployment",
			Name:      "TESTCONTROLLER",
			Namespace: namespace,
		}
	}
	// add 3 random interface stats for TESTCONTROLLER
	for i := 0; i < 3; i++ {
		stats := generateRandomInterfaceStats()
		AddInterfaceStatsToDb(stats)
	}

	//check if the data points are limited to 3
	bucket := getSubBucket(dbStats, "TESTCONTROLLER")
	if bucket == nil {
		t.Errorf("Bucket for namespace TESTCONTROLLER does not exist but should have been created!") //TODO subbucket should exist but bolt 'forgets' it
	}

}

func getSubBucket(db *bolt.DB, bucketName string) *bolt.Bucket {
	tx, err := db.Begin(false)
	if err != nil {
		log.Fatalf("Error beginning transaction: %v", err)
		return nil
	}
	defer tx.Rollback()

	mainBucket := tx.Bucket([]byte(TRAFFIC_BUCKET_NAME))
	if mainBucket == nil {
		return nil
	}

	return mainBucket.Bucket([]byte(bucketName))
}

func bucketExists(db *bolt.DB, bucketName string) bool {
	bucket := getSubBucket(db, bucketName)
	if bucket == nil {
		log.Fatalf("Error checking bucket existence: %v", bucketName)
		return false
	}
	return true
}

func generateRandomInterfaceStats() structs.InterfaceStats {
	rand.Seed(uint64(time.Now().UnixNano()))

	return structs.InterfaceStats{
		Ip:                 fmt.Sprintf("192.168.%d.%d", rand.Intn(255), rand.Intn(255)),
		PodName:            fmt.Sprintf("pod-%d", rand.Intn(1000)),
		Namespace:          fmt.Sprintf("namespace-%d", rand.Intn(100)),
		PacketsSum:         rand.Uint64(),
		TransmitBytes:      rand.Uint64(),
		ReceivedBytes:      rand.Uint64(),
		UnknownBytes:       rand.Uint64(),
		LocalTransmitBytes: rand.Uint64(),
		LocalReceivedBytes: rand.Uint64(),
		TransmitStartBytes: rand.Uint64(),
		ReceivedStartBytes: rand.Uint64(),
		StartTime:          time.Now().Add(-time.Duration(rand.Intn(1000)) * time.Hour).Format(time.RFC3339),
		CreatedAt:          time.Now().Format(time.RFC3339),
		SocketConnections: map[string]uint64{
			"conn1": rand.Uint64(),
			"conn2": rand.Uint64(),
		},
	}
}
