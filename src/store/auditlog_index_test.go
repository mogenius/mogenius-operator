package store

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"mogenius-operator/src/config"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/valkeyclient"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the indexed audit log path against a real
// Valkey/Redis instance. They are skipped unless MO_AUDIT_TEST_VALKEY_ADDR
// is set (e.g. MO_AUDIT_TEST_VALKEY_ADDR=127.0.0.1:6379).

func setupAuditTestStore(t *testing.T) {
	t.Helper()
	addr := os.Getenv("MO_AUDIT_TEST_VALKEY_ADDR")
	if addr == "" {
		t.Skip("MO_AUDIT_TEST_VALKEY_ADDR not set, skipping audit log index integration test")
	}

	str := func(s string) *string { return &s }
	cfg := config.NewConfig()
	cfg.Declare(config.ConfigDeclaration{Key: "MO_VALKEY_ADDR", DefaultValue: str(addr)})
	cfg.Declare(config.ConfigDeclaration{Key: "MO_VALKEY_PASSWORD", DefaultValue: str("")})
	cfg.Declare(config.ConfigDeclaration{Key: "MO_STATS_RETENTION_MAX_ENTRIES", DefaultValue: str("")})
	cfg.Declare(config.ConfigDeclaration{Key: "MO_STATS_RETENTION_HOURS", DefaultValue: str("")})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vc := valkeyclient.NewValkeyClient(logger, cfg)
	require.NoError(t, vc.Connect())

	valkeyClient = vc
	auditLogger = logger

	cleanAuditKeyspace(t)
	t.Cleanup(func() {
		cleanAuditKeyspace(t)
		vc.Close()
		valkeyClient = nil
	})
}

func cleanAuditKeyspace(t *testing.T) {
	t.Helper()
	require.NoError(t, valkeyClient.DeleteMultiple("audit-log*", "idx:audit-log*"))
	auditLogIndexEnsured.Store(false)
}

func writeTestAuditEntry(t *testing.T, namespace, name, pattern string, createdAt time.Time) {
	t.Helper()
	datagram := structs.Datagram{
		Id:        fmt.Sprintf("req-%s-%s", namespace, name),
		Pattern:   pattern,
		Payload:   map[string]any{"namespace": namespace, "name": name},
		CreatedAt: createdAt,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := AddToAuditLog(datagram, logger, "ok", nil, nil, nil)
	require.NoError(t, err)
}

func TestListAuditLogIndexedOrderingAndPagination(t *testing.T) {
	setupAuditTestStore(t)

	base := time.Now().Add(-time.Hour).Truncate(time.Millisecond)
	for i := range 10 {
		writeTestAuditEntry(t, "prod", fmt.Sprintf("app-%d", i), "update/workload", base.Add(time.Duration(i)*time.Second))
	}

	entries, total, err := ListAuditLog(4, 0, nil, true, "", "")
	require.NoError(t, err)
	assert.Equal(t, 10, total)
	require.Len(t, entries, 4)
	// Newest first: app-9 down to app-6.
	for i, entry := range entries {
		assert.Equal(t, fmt.Sprintf("app-%d", 9-i), entry.Name)
	}

	// Second page continues where the first ended.
	page2, total2, err := ListAuditLog(4, 4, nil, true, "", "")
	require.NoError(t, err)
	assert.Equal(t, 10, total2)
	require.Len(t, page2, 4)
	assert.Equal(t, "app-5", page2[0].Name)

	// Offset beyond total yields an empty page but the true count.
	empty, total3, err := ListAuditLog(4, 100, nil, true, "", "")
	require.NoError(t, err)
	assert.Empty(t, empty)
	assert.Equal(t, 10, total3)
}

func TestListAuditLogIndexedNamespaceAndWorkspaceFilter(t *testing.T) {
	setupAuditTestStore(t)

	base := time.Now().Add(-time.Hour)
	writeTestAuditEntry(t, "prod", "app-a", "update/workload", base)
	writeTestAuditEntry(t, "staging", "app-b", "update/workload", base.Add(time.Second))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	AddAiChatAuditLog(logger, "ai/chat", map[string]any{"question": "hi"}, nil, "", structs.User{Email: "a@b.c"}, "ws-prod")
	AddAiChatAuditLog(logger, "ai/chat", map[string]any{"question": "yo"}, nil, "", structs.User{Email: "d@e.f"}, "ws-other")

	// Workspace-scoped: only prod namespace entries plus the matching
	// workspace's AI entries — never the other workspace's.
	entries, total, err := ListAuditLog(10, 0, []string{"prod"}, false, "ws-prod", "")
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Pattern+":"+e.Workspace] = true
		assert.NotEqual(t, "ws-other", e.Workspace)
	}
	assert.True(t, names["ai/chat:ws-prod"])

	// Cluster-wide sees everything, including both AI entries.
	_, totalAll, err := ListAuditLog(10, 0, nil, true, "", "")
	require.NoError(t, err)
	assert.Equal(t, 4, totalAll)
}

func TestListAuditLogIndexedSearch(t *testing.T) {
	setupAuditTestStore(t)

	base := time.Now().Add(-time.Hour)
	writeTestAuditEntry(t, "prod", "checkout-api", "update/workload", base)
	writeTestAuditEntry(t, "prod", "billing-api", "delete/workload", base.Add(time.Second))

	entries, total, err := ListAuditLog(10, 0, nil, true, "", "checkout")
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, entries, 1)
	assert.Equal(t, "checkout-api", entries[0].Name)
}

func TestListAuditLogBackfillsLegacyEntries(t *testing.T) {
	setupAuditTestStore(t)

	base := time.Now().Add(-time.Hour)
	writeTestAuditEntry(t, "prod", "app-a", "update/workload", base)
	writeTestAuditEntry(t, "prod", "app-b", "update/workload", base.Add(time.Second))

	// Simulate a pre-index deployment: entries exist, index does not.
	require.NoError(t, valkeyClient.DeleteMultiple("idx:audit-log*"))
	auditLogIndexEnsured.Store(false)

	entries, total, err := ListAuditLog(10, 0, nil, true, "", "")
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, entries, 2)
	assert.Equal(t, "app-b", entries[0].Name)

	// The backfill marker must now exist so the scan never runs again.
	ready, err := valkeyClient.Exists(auditLogIndexReadyKey)
	require.NoError(t, err)
	assert.True(t, ready)
}

func TestAuditEventDispatcherStampsSeqInWriteOrder(t *testing.T) {
	setupAuditTestStore(t)

	received := make(chan AuditLogEntry, 16)
	prevCallback := OnAuditLogCreated
	OnAuditLogCreated = func(entry AuditLogEntry) { received <- entry }
	prevQueue, prevQuit := auditEventQueue, auditEventQuit
	startAuditEventDispatcher()
	t.Cleanup(func() {
		close(auditEventQuit)
		OnAuditLogCreated = prevCallback
		auditEventQueue, auditEventQuit = prevQueue, prevQuit
	})

	base := time.Now().Add(-time.Hour)
	for i := range 3 {
		writeTestAuditEntry(t, "prod", fmt.Sprintf("app-%d", i), "update/workload", base.Add(time.Duration(i)*time.Second))
	}

	var lastSeq int64
	bootIds := map[string]bool{}
	for i := range 3 {
		select {
		case entry := <-received:
			assert.Greater(t, entry.Seq, lastSeq, "seq must be strictly increasing")
			lastSeq = entry.Seq
			assert.NotEmpty(t, entry.BootId)
			bootIds[entry.BootId] = true
			assert.Equal(t, fmt.Sprintf("app-%d", i), entry.Name, "events must arrive in write order")
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for audit event %d", i)
		}
	}
	assert.Len(t, bootIds, 1, "all events of one process run share a boot id")
}

func TestListAuditLogHealsStaleIndexMembers(t *testing.T) {
	setupAuditTestStore(t)

	base := time.Now().Add(-time.Hour)
	writeTestAuditEntry(t, "prod", "app-a", "update/workload", base)
	writeTestAuditEntry(t, "prod", "app-b", "update/workload", base.Add(time.Second))

	// Delete one entry behind the index's back (simulates TTL expiry or
	// the per-resource limit prune).
	keys, err := valkeyClient.Keys("audit-log:prod:app-a:*")
	require.NoError(t, err)
	for _, key := range keys {
		require.NoError(t, valkeyClient.DeleteSingle(key))
	}

	entries, _, err := ListAuditLog(10, 0, nil, true, "", "")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "app-b", entries[0].Name)

	// After healing, the count reflects reality.
	_, total, err := ListAuditLog(10, 0, nil, true, "", "")
	require.NoError(t, err)
	assert.Equal(t, 1, total)
}
