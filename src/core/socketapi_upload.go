package core

import (
	"bytes"
	"fmt"
	"log/slog"
	"mogenius-operator/src/services"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"os"
	"path/filepath"
)

// Wire protocol the platform api speaks for one binary upload on a job client
// connection: an announce datagram (files/v2/upload), then the START marker,
// the raw chunks and the END marker. Every marker and ack name has to stay
// byte-for-byte identical to what the api sends and expects.
const (
	uploadFrameStart = "######START_UPLOAD######;"
	uploadFrameEnd   = "######END_UPLOAD######;"

	patternFilesUpload = "files/v2/upload"

	ackFilesUploadDatagram = "ack:files/v2/upload:datagram"
	ackFilesUploadEnd      = "ack:files/v2/upload:end"
)

// uploadReceiver holds the one binary upload that may be in flight on a single
// websocket connection. The api picks a connection per request at random, so
// every job client read loop owns its own receiver: announce, framing and
// chunks of one transfer arrive on the same connection and must never be
// paired across connections.
type uploadReceiver struct {
	logger *slog.Logger

	fileName *string
	file     *os.File
	// spooled bytes of the current transfer; once they exceed
	// maxSpoolBytes the rest is dropped and END reports the overflow
	written  int64
	tooLarge bool
	request  *services.FilesUploadRequest
	// the original announce, kept for the audit log entry written on END
	datagram *structs.Datagram

	// tempDir hosts the spooled zip. The completion callbacks are fields so
	// the state machine can be exercised without a cluster.
	tempDir       string
	maxSpoolBytes func() int64
	uploaded      func(tempZip string, request services.FilesUploadRequest) error
	audit         func(datagram structs.Datagram, err error)
}

func newUploadReceiver(logger *slog.Logger) *uploadReceiver {
	return &uploadReceiver{
		logger:  logger,
		tempDir: "/tmp",
		// the spooled zip is compressed, so it stays below the
		// uncompressed limit; 10% headroom covers stored entries
		maxSpoolBytes: func() int64 {
			limit := services.UploadMaxBytes()
			return limit + limit/10
		},
		uploaded: services.Uploaded,
		audit: func(datagram structs.Datagram, err error) {
			// uploads mutate the PVC: audit with the original datagram
			_, _ = store.AddToAuditLog(datagram, logger, err == nil, err, nil, nil)
		},
	}
}

// announce intercepts an upload announce. It returns the ack for the api and
// true when the datagram was one; any other datagram returns false and goes
// through the regular pattern dispatch.
func (self *uploadReceiver) announce(datagram structs.Datagram) (structs.Datagram, bool) {
	if datagram.Pattern != patternFilesUpload {
		return structs.Datagram{}, false
	}
	request := services.FilesUploadRequest{}
	structs.MarshalUnmarshal(&datagram, &request)
	self.request = &request
	self.datagram = &datagram
	return structs.CreateDatagramAck(ackFilesUploadDatagram, datagram.Id), true
}

// frame consumes the START/END markers and the raw chunks between them. It
// returns the acks to send (only END produces any) and true when the message
// belonged to the upload protocol; false hands the message on as a datagram.
func (self *uploadReceiver) frame(message []byte) ([]structs.Datagram, bool) {
	if bytes.HasPrefix(message, []byte(uploadFrameStart)) {
		self.open()
		return nil, true
	}
	if bytes.HasPrefix(message, []byte(uploadFrameEnd)) {
		return self.finish(), true
	}
	if self.fileName != nil {
		if self.tooLarge {
			return nil, true
		}
		if _, err := self.file.Write(message); err != nil {
			self.logger.Error("Error writing to file", "error", err)
		}
		self.written += int64(len(message))
		if self.maxSpoolBytes != nil && self.written > self.maxSpoolBytes() {
			// keep consuming the chunks until END, but stop filling the disk
			self.logger.Warn("upload exceeds the size limit, dropping the rest", "spooledBytes", self.written)
			self.tooLarge = true
			_ = self.file.Close()
			_ = os.Remove(*self.fileName)
		}
		return nil, true
	}
	return nil, false
}

// abort drops an in-flight upload. The read loop calls it when the
// connection fails: a transfer cut off between START and END never gets its
// END, and without the reset every later message on the reconnected
// connection would be appended to the spool file instead of being dispatched.
func (self *uploadReceiver) abort() {
	if self.fileName == nil && self.request == nil {
		return
	}
	self.logger.Warn("aborting in-flight file upload after connection failure")
	if self.file != nil {
		_ = self.file.Close()
	}
	if self.fileName != nil {
		_ = os.Remove(*self.fileName)
	}
	self.fileName = nil
	self.file = nil
	self.request = nil
	self.datagram = nil
	self.written = 0
	self.tooLarge = false
}

func (self *uploadReceiver) open() {
	// a START while a spool file is still open means the previous transfer
	// never saw its END; drop it instead of leaking the descriptor
	if self.file != nil {
		_ = self.file.Close()
		_ = os.Remove(*self.fileName)
	}

	self.written = 0
	self.tooLarge = false

	fileName := filepath.Join(self.tempDir, fmt.Sprintf("%s.zip", utils.NanoId()))
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		self.logger.Error("Cannot open uploadfile", "filename", fileName, "error", err)
		self.fileName = nil
		self.file = nil
		return
	}
	self.fileName = &fileName
	self.file = file
}

func (self *uploadReceiver) finish() []structs.Datagram {
	if self.file != nil {
		_ = self.file.Close()
	}

	var uploadErr error
	switch {
	case self.tooLarge:
		uploadErr = fmt.Errorf("upload exceeds the limit of %d bytes (MO_FILES_UPLOAD_MAX_BYTES)", services.UploadMaxBytes())
	case self.fileName != nil && self.request != nil:
		uploadErr = self.uploaded(*self.fileName, *self.request)
	case self.fileName == nil:
		uploadErr = fmt.Errorf("upload failed: could not open temporary file")
	}
	if uploadErr != nil {
		self.logger.Error("Error uploading file", "error", uploadErr)
	}

	if self.fileName != nil {
		_ = os.Remove(*self.fileName)
	}

	acks := []structs.Datagram{}
	if self.request != nil {
		if self.datagram != nil {
			self.audit(*self.datagram, uploadErr)
		}
		ack := structs.CreateDatagramAck(ackFilesUploadEnd, self.request.Id)
		if uploadErr != nil {
			ack.Err = uploadErr.Error()
		}
		acks = append(acks, ack)
	}

	self.fileName = nil
	self.file = nil
	self.request = nil
	self.datagram = nil
	self.written = 0
	self.tooLarge = false
	return acks
}
