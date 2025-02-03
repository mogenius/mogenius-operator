package core

import (
	"encoding/json"
	"mogenius-k8s-manager/src/crds/v1alpha1"
	"net/http"
)

func (self *httpService) addApiRoutes(mux *http.ServeMux) {
	mux.Handle("GET /workspaces", self.withRequestLogging(http.HandlerFunc(self.apiGetWorkspaces)))
	mux.Handle("GET /workspace", self.withRequestLogging(http.HandlerFunc(self.apiGetWorkspace)))
	mux.Handle("POST /workspace", self.withRequestLogging(http.HandlerFunc(self.apiPostWorkspace)))
	mux.Handle("PUT /workspace", self.withRequestLogging(http.HandlerFunc(self.apiPutWorkspace)))
	mux.Handle("DELETE /workspace", self.withRequestLogging(http.HandlerFunc(self.apiDeleteWorkspace)))
}

func (self *httpService) apiGetWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := self.api.GetAllWorkspaces()
	if err != nil {
		self.logger.Error("failed to query workloads", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(workspaces)
	if err != nil {
		self.logger.Error("failed to json encode response", "error", err)
	}
}

func (self *httpService) apiGetWorkspace(w http.ResponseWriter, r *http.Request) {
	type Args struct {
		Workspace string `json:"workspace"`
	}
	var args Args
	err := json.NewDecoder(r.Body).Decode(&args)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	workspace, err := self.api.GetWorkspace(args.Workspace)
	if err != nil {
		self.logger.Error("failed to query workloads", "error", err)
		w.WriteHeader(http.StatusNotFound)
		_, err := w.Write([]byte(`{"error":"` + err.Error() + `}"`))
		if err != nil {
			self.logger.Error("failed to write response", "error", err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(workspace)
	if err != nil {
		self.logger.Error("failed to json encode response", "error", err)
	}
}

func (self *httpService) apiPostWorkspace(w http.ResponseWriter, r *http.Request) {
	var args v1alpha1.WorkspaceSpec
	err := json.NewDecoder(r.Body).Decode(&args)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	message, err := self.api.CreateWorkspace(args.Name, args)
	if err != nil {
		self.logger.Error("failed to query workloads", "error", err)
		w.WriteHeader(http.StatusNotFound)
		_, err := w.Write([]byte(`{"error":"` + err.Error() + `}"`))
		if err != nil {
			self.logger.Error("failed to write response", "error", err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(`{"message":"` + message + `"}`))
	if err != nil {
		self.logger.Error("failed to json encode response", "error", err)
	}
}

func (self *httpService) apiPutWorkspace(w http.ResponseWriter, r *http.Request) {
	var args v1alpha1.WorkspaceSpec
	err := json.NewDecoder(r.Body).Decode(&args)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	message, err := self.api.UpdateWorkspace(args.Name, args)
	if err != nil {
		self.logger.Error("failed to query workloads", "error", err)
		w.WriteHeader(http.StatusNotFound)
		_, err := w.Write([]byte(`{"error":"` + err.Error() + `}"`))
		if err != nil {
			self.logger.Error("failed to write response", "error", err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(`{"message":"` + message + `"}`))
	if err != nil {
		self.logger.Error("failed to json encode response", "error", err)
	}
}

func (self *httpService) apiDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	type Args struct {
		Workspace string `json:"workspace"`
	}
	var args Args
	err := json.NewDecoder(r.Body).Decode(&args)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	message, err := self.api.DeleteWorkspace(args.Workspace)
	if err != nil {
		self.logger.Error("failed to query workloads", "error", err)
		w.WriteHeader(http.StatusNotFound)
		_, err := w.Write([]byte(`{"error":"` + err.Error() + `}"`))
		if err != nil {
			self.logger.Error("failed to write response", "error", err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(`{"message":"` + message + `"}`))
	if err != nil {
		self.logger.Error("failed to json encode response", "error", err)
	}
}
