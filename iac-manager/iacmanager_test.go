package iacmanager

import (
	"testing"
	"time"
)

func TestIacManager(t *testing.T) {
	Init()

	time.Sleep(3 * time.Second)

	data := PrintIacStatus()
	if len(data) < 100 {
		t.Errorf("Error getting IAC status")
	} else {
		t.Log("IAC status retrieved ✅")
	}
}
