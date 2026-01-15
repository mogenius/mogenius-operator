package core

import (
	"encoding/json"
	"io"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/utils"
	"net/http"
)

func (self *httpService) addApiRoutes(mux *http.ServeMux) {
	mux.Handle("/socketapi", self.withRequestLogging(http.HandlerFunc(self.httpSocketApi)))
	mux.HandleFunc("/api-doc", self.serveApiDocHtml)
	mux.HandleFunc("/spec.yaml", self.serveSpecYaml)
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

	result := self.socketapi.ExecuteCommandRequest(datagram)
	datagram.Payload = result

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(datagram)
	if err != nil {
		self.logger.Error("failed to json encode response", "error", err)
	}
}

func (self *httpService) serveApiDocHtml(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	html := utils.IndexHtml()
	_, err := w.Write([]byte(html))
	if err != nil {
		self.logger.Error("failed to write index.html response", "error", err)
	}
}

func (self *httpService) serveNodeStatsHtml(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	html := utils.NodeStatsHtml()
	_, err := w.Write([]byte(html))
	if err != nil {
		self.logger.Error("failed to write index.html response", "error", err)
	}
}

func (self *httpService) serveSpecYaml(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/yaml")
	data := self.generatePatterns()
	_, err := w.Write([]byte(data))
	if err != nil {
		self.logger.Error("failed to write index.html response", "error", err)
	}
}

func (self *httpService) generatePatterns() string {
	self.socketapi.AssertPatternsUnique()
	patternConfig := self.socketapi.PatternConfigs()

	data, err := json.Marshal(patternConfig)
	assert.Assert(err == nil, err)

	return string(data)
}
