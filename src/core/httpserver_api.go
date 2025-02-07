package core

import (
	"encoding/json"
	"io"
	"net/http"
)

func (self *httpService) addApiRoutes(mux *http.ServeMux) {
	mux.Handle("/socketapi", self.withRequestLogging(http.HandlerFunc(self.httpSocketApi)))
}

func (self *httpService) httpSocketApi(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"message":"failed to read request body","error":"` + err.Error() + `"}`))
		if err != nil {
			self.logger.Error("failed to write response", "error", err)
		}
		return
	}

	datagram, err := self.socketapi.ParseDatagram(body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"message":"failed to decode request body as datagram","error":"` + err.Error() + `"}`))
		if err != nil {
			self.logger.Error("failed to write response", "error", err)
		}
		return
	}
	self.logger.Info("http request for pattern", "datagram", datagram)

	result := self.socketapi.ExecuteCommandRequest(datagram, self)
	datagram.Payload = result

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(datagram)
	if err != nil {
		self.logger.Error("failed to json encode response", "error", err)
	}
}
