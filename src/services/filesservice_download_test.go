package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeFile serves chunks from content and lets a test break individual reads.
type fakeFile struct {
	content []byte
	mu      sync.Mutex
	calls   map[int64]int
	// misbehave decides per (offset, attempt) what a read does: return a
	// short read, an error, or nothing special.
	misbehave func(offset int64, attempt int) (short bool, err error)
}

func (f *fakeFile) read(ctx context.Context, offset, length int64, dst *bytes.Buffer) error {
	f.mu.Lock()
	if f.calls == nil {
		f.calls = map[int64]int{}
	}
	f.calls[offset]++
	attempt := f.calls[offset]
	f.mu.Unlock()

	if offset%downloadBlockBytes != 0 {
		return errors.New("unaligned offset")
	}
	end := min(int64(len(f.content)), offset+((length+downloadBlockBytes-1)/downloadBlockBytes)*downloadBlockBytes)
	data := f.content[offset:end]
	if f.misbehave != nil {
		short, err := f.misbehave(offset, attempt)
		if err != nil {
			return err
		}
		if short {
			data = data[:len(data)/2]
		}
	}
	dst.Write(data)
	return nil
}

func randomContent(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	rand.New(rand.NewSource(int64(n))).Read(b)
	return b
}

func withQuietLogger(t *testing.T) {
	t.Helper()
	old := serviceLogger
	serviceLogger = slog.New(slog.NewTextHandler(io.Discard, nil))
	t.Cleanup(func() { serviceLogger = old })
}

func TestStreamFileInChunksDeliversExactBytes(t *testing.T) {
	withQuietLogger(t)
	chunk := 4 * downloadBlockBytes
	for _, size := range []int{1, 100, int(downloadBlockBytes), int(chunk) - 1, int(chunk), int(chunk) + 1, 5*int(chunk) + 12345} {
		content := randomContent(t, size)
		f := &fakeFile{content: content}
		var out bytes.Buffer
		if err := streamFileInChunks(context.Background(), int64(size), chunk, f.read, &out); err != nil {
			t.Fatalf("size %d: unexpected error: %v", size, err)
		}
		if !bytes.Equal(out.Bytes(), content) {
			t.Fatalf("size %d: content differs (got %d bytes)", size, out.Len())
		}
	}
}

func TestStreamFileInChunksEmptyFileReadsNothing(t *testing.T) {
	f := &fakeFile{}
	var out bytes.Buffer
	if err := streamFileInChunks(context.Background(), 0, downloadChunkBytes, f.read, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() != 0 || len(f.calls) != 0 {
		t.Fatalf("expected no reads and no output, got %d calls, %d bytes", len(f.calls), out.Len())
	}
}

func TestStreamFileInChunksRereadsShortAndFailedChunks(t *testing.T) {
	withQuietLogger(t)
	chunk := 4 * downloadBlockBytes
	content := randomContent(t, 3*int(chunk)+999)
	f := &fakeFile{content: content, misbehave: func(offset int64, attempt int) (bool, error) {
		switch {
		case offset == chunk && attempt == 1:
			return true, nil // short read, as seen on dev
		case offset == 2*chunk && attempt <= 2:
			return false, errors.New("stream reset")
		}
		return false, nil
	}}
	var out bytes.Buffer
	if err := streamFileInChunks(context.Background(), int64(len(content)), chunk, f.read, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(out.Bytes(), content) {
		t.Fatalf("content differs after retries (got %d of %d bytes)", out.Len(), len(content))
	}
	if f.calls[chunk] != 2 || f.calls[2*chunk] != 3 {
		t.Fatalf("expected 2 and 3 reads, got %d and %d", f.calls[chunk], f.calls[2*chunk])
	}
}

func TestStreamFileInChunksFailsWithoutForwardingAShortChunk(t *testing.T) {
	withQuietLogger(t)
	chunk := 4 * downloadBlockBytes
	content := randomContent(t, 3*int(chunk))
	f := &fakeFile{content: content, misbehave: func(offset int64, attempt int) (bool, error) {
		return offset == 2*chunk, nil // the last chunk never comes back whole
	}}
	var out bytes.Buffer
	err := streamFileInChunks(context.Background(), int64(len(content)), chunk, f.read, &out)
	if err == nil {
		t.Fatal("expected an error for a chunk that stays short")
	}
	if int64(out.Len()) != 2*chunk || !bytes.Equal(out.Bytes(), content[:2*chunk]) {
		t.Fatalf("expected exactly the two good chunks before the error, got %d bytes", out.Len())
	}
	if f.calls[2*chunk] != downloadChunkAttempts {
		t.Fatalf("expected %d attempts, got %d", downloadChunkAttempts, f.calls[2*chunk])
	}
}

func TestStreamFileInChunksCutsAFileThatGrew(t *testing.T) {
	withQuietLogger(t)
	chunk := 4 * downloadBlockBytes
	content := randomContent(t, 2*int(chunk))
	announced := int64(len(content)) - 1000 // stat ran before the file grew
	f := &fakeFile{content: content}
	var out bytes.Buffer
	if err := streamFileInChunks(context.Background(), announced, chunk, f.read, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if int64(out.Len()) != announced || !bytes.Equal(out.Bytes(), content[:announced]) {
		t.Fatalf("expected exactly the announced %d bytes, got %d", announced, out.Len())
	}
}

type failingWriter struct{ after int }

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.after <= 0 {
		return 0, errors.New("peer went away")
	}
	w.after--
	return len(p), nil
}

func TestStreamFileInChunksStopsReadingWhenTheWriterFails(t *testing.T) {
	withQuietLogger(t)
	chunk := 4 * downloadBlockBytes
	content := randomContent(t, 20*int(chunk))
	var reads atomic.Int32
	f := &fakeFile{content: content}
	read := func(ctx context.Context, offset, length int64, dst *bytes.Buffer) error {
		reads.Add(1)
		return f.read(ctx, offset, length, dst)
	}
	err := streamFileInChunks(context.Background(), int64(len(content)), chunk, read, &failingWriter{after: 2})
	if err == nil {
		t.Fatal("expected the writer's error")
	}
	// give a straggling reader goroutine the chance to run before counting
	time.Sleep(50 * time.Millisecond)
	if n := reads.Load(); n > 2+downloadChunkBuffers {
		t.Fatalf("expected reading to stop shortly after the writer failed, got %d reads of 20", n)
	}
}

func TestStreamFileInChunksHonoursCancellation(t *testing.T) {
	withQuietLogger(t)
	chunk := 4 * downloadBlockBytes
	content := randomContent(t, 10*int(chunk))
	ctx, cancel := context.WithCancel(context.Background())
	f := &fakeFile{content: content, misbehave: func(offset int64, attempt int) (bool, error) {
		if offset == 3*chunk {
			cancel()
		}
		return false, nil
	}}
	err := streamFileInChunks(ctx, int64(len(content)), chunk, f.read, io.Discard)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestStreamFileInChunksRejectsUnalignedChunkSize(t *testing.T) {
	if err := streamFileInChunks(context.Background(), 10, downloadBlockBytes+1, (&fakeFile{}).read, io.Discard); err == nil {
		t.Fatal("expected an error for a chunk size that is not a block multiple")
	}
}

func TestDdChunkCommand(t *testing.T) {
	got := ddChunkCommand("/data/a file.bin", 3*downloadChunkBytes, downloadChunkBytes)
	want := []string{"dd", "if=/data/a file.bin", "bs=65536", "skip=192", "count=64"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
	// a last chunk shorter than a block still asks for one whole block
	if last := ddChunkCommand("/f", 0, 10); last[4] != "count=1" {
		t.Fatalf("expected count=1 for a 10-byte tail, got %q", last)
	}
}
