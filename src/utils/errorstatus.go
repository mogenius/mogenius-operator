package utils

import (
	"context"
	"errors"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// HttpStatusForError reports which HTTP status an error deserves, so the socket
// response can carry it instead of leaving the platform to guess from prose.
//
// The Kubernetes API already answers this precisely: a StatusError knows its own
// code, and apierrors recognises the wrapped forms. Everything else falls back
// to 400, which is what every error used to be reported as — so an unmapped
// error is no worse off than before.
//
// Deliberately never returns 401. The platform's frontend refreshes the access
// token and replays the request on 401, so reporting a cluster RBAC denial that
// way would cost a pointless retry and, on a failed refresh, log the user out.
// An RBAC denial is 403, which is also what Kubernetes itself calls it.
func HttpStatusForError(err error) int {
	if err == nil {
		return 0
	}

	// Reason first, code second. A StatusError's reason is more precise than its
	// number: NewServerTimeout carries code 500, but "try again" is a gateway
	// timeout to our callers, not an internal error.
	switch {
	case apierrors.IsNotFound(err):
		return http.StatusNotFound
	case apierrors.IsAlreadyExists(err), apierrors.IsConflict(err):
		return http.StatusConflict
	case apierrors.IsForbidden(err), apierrors.IsUnauthorized(err):
		// IsUnauthorized included on purpose: see the note above, a denial from
		// the cluster is reported as 403 rather than 401.
		return http.StatusForbidden
	case apierrors.IsInvalid(err), apierrors.IsBadRequest(err):
		return http.StatusBadRequest
	case apierrors.IsTimeout(err), apierrors.IsServerTimeout(err), errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout
	case apierrors.IsTooManyRequests(err):
		return http.StatusTooManyRequests
	case apierrors.IsServiceUnavailable(err):
		return http.StatusServiceUnavailable
	case apierrors.IsNotAcceptable(err), apierrors.IsUnsupportedMediaType(err):
		return http.StatusBadRequest
	case errors.Is(err, context.Canceled):
		// The caller went away; nothing failed on this side.
		return http.StatusRequestTimeout
	}

	// No recognised reason: forward the API server's own code, which covers the
	// statuses not enumerated above. Only real error codes -- anything else
	// would turn a failure into a success-looking status.
	if statusErr, ok := errors.AsType[*apierrors.StatusError](err); ok {
		if code := int(statusErr.ErrStatus.Code); code >= 400 && code <= 599 {
			if code == http.StatusUnauthorized {
				return http.StatusForbidden
			}
			return code
		}
	}

	// Non-Kubernetes errors: helm, git, our own validation. They were all 400
	// before and stay 400, which keeps this change additive.
	return http.StatusBadRequest
}
