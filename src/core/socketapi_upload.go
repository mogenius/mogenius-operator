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
// connection: an announce datagram (files/upload or files/v2/upload), then the
// START marker, the raw chunks and the END marker. Every marker and ack name
// has to stay byte-for-byte identical to what the api sends and expects.
const (
	uploadFrameStart = "######START_UPLOAD######;"
	uploadFrameEnd   = "######END_UPLOAD######;"

	patternFilesUpload   = "files/upload"
	patternFilesUploadV2 = "files/v2/upload"

	ackFilesUploadDatagram   = "ack:files/upload:datagram"
	ackFilesUploadEnd        = "ack:files/upload:end"
	ackFilesUploadV2Datagram = "ack:files/v2/upload:datagram"
	ackFilesUploadV2End      = "ack:files/v2/upload:end"
)

// uploadReceiver holds the one binary upload that may be in flight on a single
// websocket connection. The api picks a connection per request at random, so
// every job client read loop owns its own receiver: announce, framing and
// chunks of one transfer arrive on the same connection and must never be
// paired across connections.
//
// Either a legacy files/upload request or a files/v2/upload request is
// pending, never both; the non-nil one decides where END dispatches.
type uploadReceiver struct {
	logger *slog.Logger

	fileName  *string
	file      *os.File
	request   *services.FilesUploadRequest
	requestV2 *services.FilesUploadRequestV2
	// the original v2 announce, kept for the audit log entry written on END
	datagramV2 *structs.Datagram

	// tempDir hosts the spooled zip. The completion callbacks are fields so
	// the state machine can be exercised without a cluster.
	tempDir    string
	uploaded   func(tempZip string, request services.FilesUploadRequest) error
	uploadedV2 func(tempZip string, request services.FilesUploadRequestV2) error
	audit      func(datagram structs.Datagram, err error)
}

func newUploadReceiver(logger *slog.Logger) *uploadReceiver {
	return &uploadReceiver{
		logger:     logger,
		tempDir:    "/tmp",
		uploaded:   services.Uploaded,
		uploadedV2: services.UploadedV2,
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
	switch datagram.Pattern {
	case patternFilesUpload:
		request := services.FilesUploadRequest{}
		structs.MarshalUnmarshal(&datagram, &request)
		self.request = &request
		self.requestV2 = nil
		self.datagramV2 = nil
		return structs.CreateDatagramAck(ackFilesUploadDatagram, datagram.Id), true
	case patternFilesUploadV2:
		request := services.FilesUploadRequestV2{}
		structs.MarshalUnmarshal(&datagram, &request)
		self.requestV2 = &request
		self.datagramV2 = &datagram
		self.request = nil
		return structs.CreateDatagramAck(ackFilesUploadV2Datagram, datagram.Id), true
	}
	return structs.Datagram{}, false
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
		if _, err := self.file.Write(message); err != nil {
			self.logger.Error("Error writing to file", "error", err)
		}
		return nil, true
	}
	return nil, false
}

func (self *uploadReceiver) open() {
	// a START while a spool file is still open means the previous transfer
	// never saw its END; drop it instead of leaking the descriptor
	if self.file != nil {
		_ = self.file.Close()
		_ = os.Remove(*self.fileName)
	}

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
	case self.fileName != nil && self.request != nil:
		uploadErr = self.uploaded(*self.fileName, *self.request)
	case self.fileName != nil && self.requestV2 != nil:
		uploadErr = self.uploadedV2(*self.fileName, *self.requestV2)
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
		ack := structs.CreateDatagramAck(ackFilesUploadEnd, self.request.Id)
		if uploadErr != nil {
			ack.Err = uploadErr.Error()
		}
		acks = append(acks, ack)
	}
	if self.requestV2 != nil {
		if self.datagramV2 != nil {
			self.audit(*self.datagramV2, uploadErr)
		}
		ack := structs.CreateDatagramAck(ackFilesUploadV2End, self.requestV2.Id)
		if uploadErr != nil {
			ack.Err = uploadErr.Error()
		}
		acks = append(acks, ack)
	}

	self.fileName = nil
	self.file = nil
	self.request = nil
	self.requestV2 = nil
	self.datagramV2 = nil
	return acks
}
