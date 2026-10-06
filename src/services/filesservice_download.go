package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mogenius-operator/src/dtos"
	mokubernetes "mogenius-operator/src/kubernetes"
	"path"
	"strconv"
	"time"
)

// Regular files are streamed in chunks, each read by its own short exec and
// checked for its length before a single byte of it is forwarded.
//
// One long `cat` per file lost the last few hundred KB to a few MB of large
// downloads (MOG-4735): client-go's exec only logs a failed stdout copy,
// discards the rest and still reports success, so the stream ended with a
// clean END and the browser got a file that was short at the end. A chunk
// that comes back short is now read again; nothing short ever reaches the
// browser, and a chunk that keeps failing ends the stream with an error.
//
// Chunks start on block boundaries so `dd` can seek with plain skip/count,
// which GNU coreutils and busybox both understand. Two buffers circulate: one
// is forwarded while the next is read, so the exec latency hides behind the
// download and memory stays at two chunks per stream.
const (
	downloadChunkBytes    int64 = 4 << 20
	downloadBlockBytes    int64 = 64 << 10
	downloadChunkAttempts       = 3
	downloadChunkBuffers        = 2
	downloadRetryDelay          = 500 * time.Millisecond
)

// downloadChunkReader writes up to length bytes of the file, starting at
// offset, into dst.
type downloadChunkReader func(ctx context.Context, offset, length int64, dst *bytes.Buffer) error

// DownloadToWriter streams what info describes into w, from info.Offset on.
// Regular files go in verified chunks; folders and anything else that is not
// a regular file (symlink, fifo, device) stay one exec, because their output
// has no length to check against. The size from info is what the browser was
// promised, so a file that grew since is cut there. Cancelling ctx ends the
// running exec.
func DownloadToWriter(ctx context.Context, pfile dtos.PvcFileRequestDto, info FilesDownloadStreamInfo, w io.Writer) error {
	target, err := resolveFileTarget(pfile)
	if err != nil {
		return err
	}
	containerPath, err := resolvePath(target.MountRoot, pfile.Path)
	if err != nil {
		return err
	}

	switch {
	case info.IsDirectory:
		command := []string{"tar", "czf", "-", "-C", path.Dir(containerPath), path.Base(containerPath)}
		return mokubernetes.ExecInPodToWriterContext(ctx, target.Namespace, target.Pod, target.Container, command, nil, w)
	case !info.Resumable:
		return mokubernetes.ExecInPodToWriterContext(ctx, target.Namespace, target.Pod, target.Container, []string{"cat", containerPath}, nil, w)
	}

	readChunk := func(ctx context.Context, offset, length int64, dst *bytes.Buffer) error {
		return mokubernetes.ExecInPodToWriterContext(
			ctx, target.Namespace, target.Pod, target.Container,
			ddChunkCommand(containerPath, offset, length), nil, dst,
		)
	}
	return streamFileInChunks(ctx, info.Offset, info.SizeInBytes, downloadChunkBytes, readChunk, w)
}

// ddChunkCommand reads length bytes from offset; offset must be a multiple of
// downloadBlockBytes. The last chunk of a file asks for a whole block and
// simply ends at EOF.
func ddChunkCommand(containerPath string, offset, length int64) []string {
	blocks := (length + downloadBlockBytes - 1) / downloadBlockBytes
	return []string{
		"dd",
		"if=" + containerPath,
		"bs=" + strconv.FormatInt(downloadBlockBytes, 10),
		"skip=" + strconv.FormatInt(offset/downloadBlockBytes, 10),
		"count=" + strconv.FormatInt(blocks, 10),
	}
}

// streamFileInChunks writes the bytes from start up to size into w, read
// chunk by chunk. Chunks stay on block boundaries; when start lies inside a
// block, the first chunk begins at that block and its leading bytes are
// dropped. A reader goroutine fills the next chunk while the current one is
// written; it stops as soon as the writer gives up or ctx ends.
func streamFileInChunks(ctx context.Context, start, size, chunkBytes int64, readChunk downloadChunkReader, w io.Writer) error {
	if start < 0 || start > size {
		return fmt.Errorf("download start %d is outside the file (%d bytes)", start, size)
	}
	if start == size {
		return nil
	}
	if chunkBytes <= 0 || chunkBytes%downloadBlockBytes != 0 {
		return fmt.Errorf("download chunk size %d is not a positive multiple of %d", chunkBytes, downloadBlockBytes)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type chunk struct {
		buf  *bytes.Buffer
		skip int
		err  error
	}
	free := make(chan *bytes.Buffer, downloadChunkBuffers)
	for range downloadChunkBuffers {
		free <- bytes.NewBuffer(make([]byte, 0, chunkBytes+downloadBlockBytes))
	}
	ready := make(chan chunk)

	go func() {
		defer close(ready)
		for offset := start - start%downloadBlockBytes; offset < size; offset += chunkBytes {
			var buf *bytes.Buffer
			select {
			case buf = <-free:
			case <-ctx.Done():
				return
			}
			err := readChunkVerified(ctx, readChunk, offset, min(chunkBytes, size-offset), buf)
			select {
			case ready <- chunk{buf: buf, skip: int(max(0, start-offset)), err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()

	var written int64
	for c := range ready {
		if c.err != nil {
			return c.err
		}
		n, err := w.Write(c.buf.Bytes()[c.skip:])
		written += int64(n)
		if err != nil {
			return err
		}
		free <- c.buf
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if written != size-start {
		return fmt.Errorf("download wrote %d of %d bytes", written, size-start)
	}
	return nil
}

// readChunkVerified reads one chunk into buf and accepts it only at full
// length. Bytes beyond length (the file grew after stat) are dropped: the
// download announced size bytes and delivers exactly those.
func readChunkVerified(ctx context.Context, readChunk downloadChunkReader, offset, length int64, buf *bytes.Buffer) error {
	var lastErr error
	for attempt := 1; attempt <= downloadChunkAttempts; attempt++ {
		buf.Reset()
		err := readChunk(ctx, offset, length, buf)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		switch {
		case err != nil:
			lastErr = fmt.Errorf("read bytes %d-%d: %w", offset, offset+length-1, err)
		case int64(buf.Len()) < length:
			lastErr = fmt.Errorf("read bytes %d-%d: got %d of %d bytes", offset, offset+length-1, buf.Len(), length)
		default:
			buf.Truncate(int(length))
			return nil
		}
		if attempt == downloadChunkAttempts {
			break
		}
		serviceLogger.Warn("download chunk incomplete, reading it again", "attempt", attempt, "error", lastErr)
		select {
		case <-time.After(time.Duration(attempt) * downloadRetryDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return lastErr
}
