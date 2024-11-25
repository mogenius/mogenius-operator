package dbstats_test

import (
	"fmt"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/dbstats"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"math/rand/v2"

	"go.etcd.io/bbolt"
)

// test the functionality of the custom resource with a basic pod
func TestAddInterfaceStatsToDbCreateDBs(t *testing.T) {
	stat := generateRandomInterfaceStats()

	logManager := logging.NewMockSlogManager(t)
	config := cfg.NewConfig()
	dbstats.Setup(logManager, config)
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_STATS_PATH",
		DefaultValue: utils.Pointer(filepath.Join(t.TempDir(), "test01.db")),
	})
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_STATS_MAX_DATA_POINTS",
		DefaultValue: utils.Pointer("1000"),
	})

	dbstats.GetControllerFunc = func(namespace string, podName string) *kubernetes.K8sController {
		return &kubernetes.K8sController{
			Kind:      "Deployment",
			Name:      podName,
			Namespace: namespace,
		}
	}
	dbstats.Start()

	tx, err := dbstats.DbStats.Begin(false)
	assert.Assert(err == nil, err)

	err = tx.Rollback()
	assert.Assert(err == nil, err)
	dbstats.AddInterfaceStatsToDb(stat)

	tx, err = dbstats.DbStats.Begin(false)
	assert.Assert(err == nil, err)
	assert.Assert(
		bucketExists(tx, stat.Namespace),
		fmt.Sprintf("Bucket for namespace %s does not exist but should have been created!", stat.Namespace),
	)
	err = tx.Rollback()
	assert.Assert(err == nil, err)
}

func TestAddInterfaceStatsToDbLimitDataPoints(t *testing.T) {
	logManager := logging.NewMockSlogManager(t)
	config := config.NewConfig()
	dbstats.Setup(logManager, config)
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_STATS_PATH",
		DefaultValue: utils.Pointer(filepath.Join(t.TempDir(), "test02.db")),
	})
	config.Declare(cfg.ConfigDeclaration{
		Key:          "MO_BBOLT_DB_STATS_MAX_DATA_POINTS",
		DefaultValue: utils.Pointer("3"),
	})

	dbstats.Start()

	dbstats.GetControllerFunc = func(namespace string, podName string) *kubernetes.K8sController {
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
		dbstats.AddInterfaceStatsToDb(stats)
	}

	tx, err := dbstats.DbStats.Begin(false)
	t.Cleanup(func() {
		err := tx.Rollback()
		if err != nil {
			t.Error(err)
		}
	})
	assert.Assert(err == nil, err)

	//check if the data points are limited to 3
	bucket := getNestedBucket(tx, []string{"TESTNS", "TESTCONTROLLER"})
	assert.Assert(bucket != nil, "Bucket for namespace TESTCONTROLLER does not exist but should have been created!")

	maxDataPoints, err := strconv.Atoi(config.Get("MO_BBOLT_DB_STATS_MAX_DATA_POINTS"))
	assert.Assert(err == nil, err)
	assert.Assert(bucket.Stats().KeyN == maxDataPoints+1, fmt.Sprintf("Expected %d data points but got %d", maxDataPoints, bucket.Stats().KeyN))
}

func getNestedBucket(tx *bbolt.Tx, bucketChain []string) *bbolt.Bucket {
	mainBucket := tx.Bucket([]byte(dbstats.TRAFFIC_BUCKET_NAME))
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

func bucketExists(tx *bbolt.Tx, bucketName string) bool {
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
