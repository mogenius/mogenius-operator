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

	utils.CONFIG.Kubernetes.BboltDbStatsPath = "/tmp/test01.db"
	utils.CONFIG.Stats.MaxDataPoints = 1000

	getControllerFunc = func(namespace string, podName string) *kubernetes.K8sController {
		return &kubernetes.K8sController{
			Kind:      "Deployment",
			Name:      podName,
			Namespace: namespace,
		}
	}
	Init()

	tx, err := dbStats.Begin(false)
	if err != nil {
		t.Errorf("Error beginning transaction: %v", err)
	}

	// check if db has a bucket for the namespace
	if !bucketExists(tx, stat.Namespace) {
		log.Printf("Bucket for namespace %s does not exist and should be created once the stat is added", stat.Namespace)
	}
	tx.Rollback()
	AddInterfaceStatsToDb(stat)

	tx, err = dbStats.Begin(false)
	if err != nil {
		t.Errorf("Error beginning transaction: %v", err)
	}
	if !bucketExists(tx, stat.Namespace) {
		t.Errorf("Bucket for namespace %s does not exist but should have been created!", stat.Namespace)
	}
	tx.Rollback()
}

func TestAddInterfaceStatsToDbLimitDataPoints(t *testing.T) {
	utils.CONFIG.Kubernetes.BboltDbStatsPath = "/tmp/test02.db"
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
	for i := 0; i < 20; i++ {
		stats := generateRandomInterfaceStats()
		stats.Namespace = "TESTNS"
		AddInterfaceStatsToDb(stats)
	}

	tx, err := dbStats.Begin(false)
	if err != nil {
		t.Errorf("Error beginning transaction: %v", err)
	}
	defer tx.Rollback()

	//check if the data points are limited to 3
	bucket := getNestedBucket(tx, []string{"TESTNS", "TESTCONTROLLER"})
	if bucket == nil {
		t.Errorf("Bucket for namespace TESTCONTROLLER does not exist but should have been created!") //TODO subbucket should exist but bolt 'forgets' it
	}

	if bucket.Stats().KeyN != utils.CONFIG.Stats.MaxDataPoints+1 {
		t.Errorf("Expected %d data points but got %d", utils.CONFIG.Stats.MaxDataPoints, bucket.Stats().KeyN)
	}

}

func readSubBucketContents(tx *bolt.Tx, bucketChain []string) {
	bucket := getNestedBucket(tx, bucketChain)
	if bucket == nil {
		log.Printf("Bucket %v does not exist", bucketChain)
		// return nil
	}

	err := bucket.ForEach(func(k, v []byte) error {
		log.Printf("Key: %s, Value: %s", k, v)
		return nil
	})
	if err != nil {
		log.Printf("Error reading bucket contents: %v", err)
		// return err
	}
	// return nil
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
	if bucket == nil {
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
