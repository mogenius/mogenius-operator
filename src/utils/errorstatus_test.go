package utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestHttpStatusForError(t *testing.T) {
	secrets := schema.GroupResource{Group: "", Resource: "secrets"}

	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil has no status", nil, 0},
		{"not found", apierrors.NewNotFound(secrets, "write-token"), http.StatusNotFound},
		{"already exists", apierrors.NewAlreadyExists(secrets, "write-token"), http.StatusConflict},
		{"conflict", apierrors.NewConflict(secrets, "write-token", errors.New("changed")), http.StatusConflict},
		{"forbidden", apierrors.NewForbidden(secrets, "write-token", errors.New("rbac")), http.StatusForbidden},
		{"bad request", apierrors.NewBadRequest("nope"), http.StatusBadRequest},
		{"server timeout", apierrors.NewServerTimeout(secrets, "get", 1), http.StatusGatewayTimeout},
		{"too many requests", apierrors.NewTooManyRequestsError("slow down"), http.StatusTooManyRequests},
		{"service unavailable", apierrors.NewServiceUnavailable("no backend"), http.StatusServiceUnavailable},
		{"deadline exceeded", context.DeadlineExceeded, http.StatusGatewayTimeout},
		{"canceled", context.Canceled, http.StatusRequestTimeout},
		{"a plain error stays a bad request", errors.New("helm: chart not loadable"), http.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := HttpStatusForError(test.err); got != test.want {
				t.Fatalf("HttpStatusForError(%v) = %d, want %d", test.err, got, test.want)
			}
		})
	}
}

// Wrapped errors are the normal case: handlers add context with %w before
// returning, and a mapping that only recognised bare errors would report almost
// everything as 400.
func TestHttpStatusForWrappedError(t *testing.T) {
	secrets := schema.GroupResource{Group: "", Resource: "secrets"}
	wrapped := fmt.Errorf("failed to read the write credential: %w", apierrors.NewNotFound(secrets, "write-token"))

	if got := HttpStatusForError(wrapped); got != http.StatusNotFound {
		t.Fatalf("wrapped NotFound = %d, want %d", got, http.StatusNotFound)
	}
}

// The platform's frontend refreshes the token and replays the request on 401, so
// a cluster denial must never be reported that way.
func TestUnauthorizedIsReportedAsForbidden(t *testing.T) {
	if got := HttpStatusForError(apierrors.NewUnauthorized("token expired")); got != http.StatusForbidden {
		t.Fatalf("Unauthorized = %d, want %d", got, http.StatusForbidden)
	}

	statusErr := &apierrors.StatusError{ErrStatus: metav1.Status{Code: http.StatusUnauthorized, Message: "nope"}}
	if got := HttpStatusForError(statusErr); got != http.StatusForbidden {
		t.Fatalf("StatusError 401 = %d, want %d", got, http.StatusForbidden)
	}
}

// A StatusError knows its own code; anything in the error range is forwarded
// rather than squeezed into the handful of cases enumerated above.
func TestStatusErrorCodeIsForwarded(t *testing.T) {
	for _, code := range []int32{http.StatusNotFound, http.StatusGone, http.StatusInternalServerError, 418} {
		statusErr := &apierrors.StatusError{ErrStatus: metav1.Status{Code: code}}
		if got := HttpStatusForError(statusErr); got != int(code) {
			t.Fatalf("StatusError %d = %d, want %d", code, got, code)
		}
	}
}

// A code outside the error range would turn a failure into a success-looking
// status, so it falls through to the default instead.
func TestNonErrorStatusCodeFallsBack(t *testing.T) {
	statusErr := &apierrors.StatusError{ErrStatus: metav1.Status{Code: 0, Message: "no code"}}
	if got := HttpStatusForError(statusErr); got != http.StatusBadRequest {
		t.Fatalf("StatusError without a code = %d, want %d", got, http.StatusBadRequest)
	}
}
