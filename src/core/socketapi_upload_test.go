package core

import (
	"errors"
	"io"
	"log/slog"
	"mogenius-operator/src/services"
	"mogenius-operator/src/structs"
	"os"
	"path/filepath"
	"testing"
)

// testUploadReceiver records the completion callbacks instead of touching a
// cluster, and spools into the test's temp dir.
type testUploadReceiver struct {
	*uploadReceiver
	legacyCalls []services.FilesUploadRequest
	v2Calls     []services.FilesUploadRequestV2
	spooled     []string
	audits      []error
	uploadErr   error
}

func newTestUploadReceiver(t *testing.T) *testUploadReceiver {
	t.Helper()
	r := &testUploadReceiver{
		uploadReceiver: newUploadReceiver(slog.New(slog.NewTextHandler(io.Discard, nil))),
	}
	r.tempDir = t.TempDir()
	r.uploaded = func(tempZip string, request services.FilesUploadRequest) error {
		r.legacyCalls = append(r.legacyCalls, request)
		r.spooled = append(r.spooled, readFile(t, tempZip))
		return r.uploadErr
	}
	r.uploadedV2 = func(tempZip string, request services.FilesUploadRequestV2) error {
		r.v2Calls = append(r.v2Calls, request)
		r.spooled = append(r.spooled, readFile(t, tempZip))
		return r.uploadErr
	}
	r.audit = func(datagram structs.Datagram, err error) {
		r.audits = append(r.audits, err)
	}
	return r
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("spool file %s not readable: %v", path, err)
	}
	return string(content)
}

func announceV2(id string, transferId string) structs.Datagram {
	return structs.Datagram{
		Id:      id,
		Pattern: patternFilesUploadV2,
		Payload: map[string]any{
			"file":        map[string]any{"namespace": "ns", "pvcName": "data", "path": "/"},
			"sizeInBytes": 11,
			"id":          transferId,
		},
	}
}

func announceLegacy(id string, transferId string) structs.Datagram {
	return structs.Datagram{
		Id:      id,
		Pattern: patternFilesUpload,
		Payload: map[string]any{
			"file":        map[string]any{"volumeNamespace": "ns", "volumeName": "vol", "path": "/"},
			"sizeInBytes": 11,
			"id":          transferId,
		},
	}
}

func spoolEntries(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("temp dir not readable: %v", err)
	}
	return len(entries)
}

func TestUploadReceiverV2Flow(t *testing.T) {
	r := newTestUploadReceiver(t)

	ack, ok := r.announce(announceV2("dg-1", "transfer-1"))
	if !ok {
		t.Fatal("files/v2/upload announce was not intercepted")
	}
	if ack.Pattern != ackFilesUploadV2Datagram || ack.Id != "dg-1" {
		t.Fatalf("announce ack = %q/%q, want %q/dg-1", ack.Pattern, ack.Id, ackFilesUploadV2Datagram)
	}

	if acks, consumed := r.frame([]byte(uploadFrameStart)); !consumed || len(acks) != 0 {
		t.Fatalf("START: consumed=%v acks=%d, want consumed and no acks", consumed, len(acks))
	}
	for _, chunk := range []string{"hello ", "world"} {
		if acks, consumed := r.frame([]byte(chunk)); !consumed || len(acks) != 0 {
			t.Fatalf("chunk %q: consumed=%v acks=%d, want consumed and no acks", chunk, consumed, len(acks))
		}
	}
	acks, consumed := r.frame([]byte(uploadFrameEnd))
	if !consumed || len(acks) != 1 {
		t.Fatalf("END: consumed=%v acks=%d, want consumed and one ack", consumed, len(acks))
	}
	if acks[0].Pattern != ackFilesUploadV2End || acks[0].Id != "transfer-1" || acks[0].Err != "" {
		t.Fatalf("end ack = %+v, want %s for transfer-1 without error", acks[0], ackFilesUploadV2End)
	}

	if len(r.v2Calls) != 1 || r.v2Calls[0].Id != "transfer-1" || r.v2Calls[0].File.PvcName != "data" {
		t.Fatalf("uploadedV2 calls = %+v, want one for transfer-1 on pvc data", r.v2Calls)
	}
	if len(r.legacyCalls) != 0 {
		t.Fatalf("legacy upload must not run for a v2 announce, got %+v", r.legacyCalls)
	}
	if r.spooled[0] != "hello world" {
		t.Fatalf("spooled content = %q, want chunks concatenated in order", r.spooled[0])
	}
	if len(r.audits) != 1 || r.audits[0] != nil {
		t.Fatalf("audit calls = %v, want exactly one success entry", r.audits)
	}
	if n := spoolEntries(t, r.tempDir); n != 0 {
		t.Fatalf("%d spool files left behind, want none", n)
	}

	// state is reset: a stray chunk is regular traffic again
	if _, consumed := r.frame([]byte(`{"id":"x"}`)); consumed {
		t.Fatal("receiver still consumed messages after END")
	}
}

func TestUploadReceiverLegacyFlow(t *testing.T) {
	r := newTestUploadReceiver(t)

	ack, ok := r.announce(announceLegacy("dg-2", "transfer-2"))
	if !ok || ack.Pattern != ackFilesUploadDatagram || ack.Id != "dg-2" {
		t.Fatalf("legacy announce: ok=%v ack=%+v, want %s for dg-2", ok, ack, ackFilesUploadDatagram)
	}

	r.frame([]byte(uploadFrameStart))
	r.frame([]byte("legacy bytes"))
	acks, _ := r.frame([]byte(uploadFrameEnd))

	if len(acks) != 1 || acks[0].Pattern != ackFilesUploadEnd || acks[0].Id != "transfer-2" {
		t.Fatalf("end acks = %+v, want one %s for transfer-2", acks, ackFilesUploadEnd)
	}
	if len(r.legacyCalls) != 1 || r.legacyCalls[0].File.VolumeName != "vol" || r.spooled[0] != "legacy bytes" {
		t.Fatalf("legacy upload = %+v / %q, want one call for volume vol with the spooled bytes", r.legacyCalls, r.spooled)
	}
	if len(r.v2Calls) != 0 || len(r.audits) != 0 {
		t.Fatalf("v2 upload/audit must not run for a legacy announce (v2=%d audits=%d)", len(r.v2Calls), len(r.audits))
	}
}

func TestUploadReceiverUploadErrorEndsUpInAck(t *testing.T) {
	r := newTestUploadReceiver(t)
	r.uploadErr = errors.New("pvc is not mounted")

	r.announce(announceV2("dg-3", "transfer-3"))
	r.frame([]byte(uploadFrameStart))
	r.frame([]byte("x"))
	acks, _ := r.frame([]byte(uploadFrameEnd))

	if len(acks) != 1 || acks[0].Err != "pvc is not mounted" {
		t.Fatalf("end acks = %+v, want the upload error in ack.Err", acks)
	}
	if len(r.audits) != 1 || r.audits[0] == nil {
		t.Fatalf("audit calls = %v, want one failure entry", r.audits)
	}
}

func TestUploadReceiverUnwritableSpoolReportsError(t *testing.T) {
	r := newTestUploadReceiver(t)
	r.tempDir = filepath.Join(r.tempDir, "does", "not", "exist")

	r.announce(announceV2("dg-4", "transfer-4"))
	if _, consumed := r.frame([]byte(uploadFrameStart)); !consumed {
		t.Fatal("START must be consumed even when the spool file cannot be opened")
	}
	acks, _ := r.frame([]byte(uploadFrameEnd))

	if len(acks) != 1 || acks[0].Id != "transfer-4" || acks[0].Err != "upload failed: could not open temporary file" {
		t.Fatalf("end acks = %+v, want an ack for transfer-4 carrying the open error", acks)
	}
	if len(r.v2Calls) != 0 {
		t.Fatalf("uploadedV2 must not run without a spool file, got %+v", r.v2Calls)
	}
}

func TestUploadReceiverFramesWithoutAnnounceSendNoAck(t *testing.T) {
	r := newTestUploadReceiver(t)

	r.frame([]byte(uploadFrameStart))
	r.frame([]byte("orphan"))
	acks, consumed := r.frame([]byte(uploadFrameEnd))

	if !consumed || len(acks) != 0 {
		t.Fatalf("END without announce: consumed=%v acks=%+v, want consumed and no acks", consumed, acks)
	}
	if len(r.legacyCalls)+len(r.v2Calls) != 0 {
		t.Fatal("no upload may run without an announce")
	}
	if n := spoolEntries(t, r.tempDir); n != 0 {
		t.Fatalf("%d spool files left behind, want none", n)
	}
}

// Each connection owns its receiver: an announce on one connection must not
// be completed by frames on another. That was the shape of the original bug.
func TestUploadReceiverStateIsPerConnection(t *testing.T) {
	a := newTestUploadReceiver(t)
	b := newTestUploadReceiver(t)

	a.announce(announceV2("dg-5", "transfer-5"))

	b.frame([]byte(uploadFrameStart))
	b.frame([]byte("wrong socket"))
	if acks, _ := b.frame([]byte(uploadFrameEnd)); len(acks) != 0 {
		t.Fatalf("connection b acked a transfer announced on connection a: %+v", acks)
	}
	if len(b.v2Calls) != 0 {
		t.Fatal("connection b ran an upload announced on connection a")
	}

	a.frame([]byte(uploadFrameStart))
	a.frame([]byte("right socket"))
	acks, _ := a.frame([]byte(uploadFrameEnd))
	if len(acks) != 1 || acks[0].Id != "transfer-5" || a.spooled[0] != "right socket" {
		t.Fatalf("connection a: acks=%+v spooled=%v, want its own transfer completed", acks, a.spooled)
	}
}

func TestUploadReceiverIgnoresRegularTraffic(t *testing.T) {
	r := newTestUploadReceiver(t)

	if _, ok := r.announce(structs.Datagram{Id: "dg-6", Pattern: "files/v2/list"}); ok {
		t.Fatal("a non-upload pattern was intercepted as announce")
	}
	if _, consumed := r.frame([]byte(`{"id":"dg-6","pattern":"files/v2/list"}`)); consumed {
		t.Fatal("a datagram was consumed as upload chunk while no upload is in flight")
	}
}
