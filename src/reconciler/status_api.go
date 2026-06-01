package reconciler

import "time"

type Status struct {
	IsActive   bool           `json:"is_active"`
	LastUpdate *time.Time     `json:"last_update,omitempty"`
	Results    []StatusResult `json:"results"`
}

type StatusResult struct {
	ResourceKind      string          `json:"resource_kind"`
	ResourceName      string          `json:"resource_name"`
	ResourceNamespace string          `json:"resource_namespace"`
	Messages          []StatusMessage `json:"messages"`
}

type StatusMessage struct {
	Message string `json:"message"`
}

// Status returns a snapshot of the current reconciler state.
func (r *genericReconciler) Status() Status {
	r.statusMu.RLock()
	defer r.statusMu.RUnlock()

	results := []StatusResult{}
	for _, v := range r.objectState {
		statusResult := StatusResult{
			ResourceKind:      v.ResourceKind,
			ResourceName:      v.ResourceName,
			ResourceNamespace: v.ResourceNamespace,
			Messages:          []StatusMessage{},
		}
		for _, r := range v.Result {
			if r.Err != nil {
				msg := r.Err.Error()
				if r.IsWarning {
					msg = "WARNING: " + msg
				}
				statusResult.Messages = append(statusResult.Messages, StatusMessage{Message: msg})
			}
		}
		results = append(results, statusResult)
	}

	s := Status{
		IsActive:   r.active.Load(),
		LastUpdate: r.lastUpdate,
		Results:    results,
	}
	return s
}
