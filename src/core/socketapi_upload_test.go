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
	calls     []services.FilesUploadRequest
	spooled   []string
	audits    []error
	uploadErr error
}

func newTestUploadReceiver(t *testing.T) *testUploadReceiver {
	t.Helper()
	r := &testUploadReceiver{
		uploadReceiver: newUploadReceiver(slog.New(slog.NewTextHandler(io.Discard, nil))),
	}
	r.tempDir = t.TempDir()
	r.uploaded = func(tempZip string, request services.FilesUploadRequest) error {
		r.calls = append(r.calls, request)
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

func announceUpload(id string, transferId string) structs.Datagram {
	return structs.Datagram{
		Id:      id,
		Pattern: patternFilesUpload,
		Payload: map[string]any{
			"file":        map[string]any{"namespace": "ns", "pvcName": "data", "path": "/"},
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

func TestUploadReceiverFlow(t *testing.T) {
	r := newTestUploadReceiver(t)

	ack, ok := r.announce(announceUpload("dg-1", "transfer-1"))
	if !ok {
		t.Fatal("files/v2/upload announce was not intercepted")
	}
	if ack.Pattern != ackFilesUploadDatagram || ack.Id != "dg-1" {
		t.Fatalf("announce ack = %q/%q, want %q/dg-1", ack.Pattern, ack.Id, ackFilesUploadDatagram)
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
	if acks[0].Pattern != ackFilesUploadEnd || acks[0].Id != "transfer-1" || acks[0].Err != "" {
		t.Fatalf("end ack = %+v, want %s for transfer-1 without error", acks[0], ackFilesUploadEnd)
	}

	if len(r.calls) != 1 || r.calls[0].Id != "transfer-1" || r.calls[0].File.PvcName != "data" {
		t.Fatalf("uploaded calls = %+v, want one for transfer-1 on pvc data", r.calls)
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

func TestUploadReceiverUploadErrorEndsUpInAck(t *testing.T) {
	r := newTestUploadReceiver(t)
	r.uploadErr = errors.New("pvc is not mounted")

	r.announce(announceUpload("dg-3", "transfer-3"))
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

	r.announce(announceUpload("dg-4", "transfer-4"))
	if _, consumed := r.frame([]byte(uploadFrameStart)); !consumed {
		t.Fatal("START must be consumed even when the spool file cannot be opened")
	}
	acks, _ := r.frame([]byte(uploadFrameEnd))

	if len(acks) != 1 || acks[0].Id != "transfer-4" || acks[0].Err != "upload failed: could not open temporary file" {
		t.Fatalf("end acks = %+v, want an ack for transfer-4 carrying the open error", acks)
	}
	if len(r.calls) != 0 {
		t.Fatalf("uploaded must not run without a spool file, got %+v", r.calls)
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
	if len(r.calls) != 0 {
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

	a.announce(announceUpload("dg-5", "transfer-5"))

	b.frame([]byte(uploadFrameStart))
	b.frame([]byte("wrong socket"))
	if acks, _ := b.frame([]byte(uploadFrameEnd)); len(acks) != 0 {
		t.Fatalf("connection b acked a transfer announced on connection a: %+v", acks)
	}
	if len(b.calls) != 0 {
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
