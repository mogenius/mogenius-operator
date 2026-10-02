package dtos

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

// Exactly one of PvcName and Pod must be set; the validator tags carry that
// rule. The validator is built here directly: utils.ValidateJSON logs through
// a logger that only Setup initialises.
func TestPvcFileRequestDtoRequiresPvcOrPod(t *testing.T) {
	validate := validator.New(validator.WithRequiredStructEnabled())
	ok := []PvcFileRequestDto{
		{Namespace: "ns", PvcName: "data", Path: "/"},
		{Namespace: "ns", Pod: "sb-1", Path: "/home"},
		{Namespace: "ns", Pod: "sb-1", Container: "sandbox", Path: "/home"},
	}
	for _, dto := range ok {
		if err := validate.Struct(dto); err != nil {
			t.Errorf("%+v rejected: %v", dto, err)
		}
	}
	bad := []PvcFileRequestDto{
		{Namespace: "ns", Path: "/"},
		{Namespace: "ns", Pod: "sb-1"},
		{Pod: "sb-1", Path: "/"},
	}
	for _, dto := range bad {
		if err := validate.Struct(dto); err == nil {
			t.Errorf("%+v accepted", dto)
		}
	}
	if !(PvcFileRequestDto{Pod: "x"}).AddressesPod() || (PvcFileRequestDto{PvcName: "x"}).AddressesPod() {
		t.Error("AddressesPod wrong")
	}
}
