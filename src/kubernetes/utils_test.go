package kubernetes

import (
	"testing"
)

func TestParseDfOutput(t *testing.T) {
	t.Run("parses POSIX df -P -k output in 1K blocks", func(t *testing.T) {
		output := "Filesystem     1024-blocks   Used Available Capacity Mounted on\n" +
			"overlay        5242880        100   5142780       1% /exports\n"
		free, used, total, err := parseDfOutput(output)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 5242880*1024 || used != 100*1024 || free != 5142780*1024 {
			t.Fatalf("unexpected values: free=%d used=%d total=%d", free, used, total)
		}
	})

	t.Run("parses busybox df -P -k header variant", func(t *testing.T) {
		output := "Filesystem           1K-blocks      Used Available Use% Mounted on\n" +
			"/dev/sda1              1000000    250000    750000  25% /data\n"
		free, used, total, err := parseDfOutput(output)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 1000000*1024 || used != 250000*1024 || free != 750000*1024 {
			t.Fatalf("unexpected values: free=%d used=%d total=%d", free, used, total)
		}
	})

	t.Run("uses the last line of multi-line output", func(t *testing.T) {
		output := "Filesystem     1024-blocks   Used Available Capacity Mounted on\n" +
			"tmpfs          999  1  998   1% /other\n" +
			"/dev/sda1      1000  200  800   20% /data\n"
		free, used, total, err := parseDfOutput(output)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 1000*1024 || used != 200*1024 || free != 800*1024 {
			t.Fatalf("unexpected values: free=%d used=%d total=%d", free, used, total)
		}
	})

	t.Run("handles a wrapped long device name (numbers on their own line)", func(t *testing.T) {
		output := "Filesystem     1K-blocks    Used Available Use% Mounted on\n" +
			"192.168.1.10:/srv/nfs/default-data-tandoor-postgresql-0-pvc-a1242d44\n" +
			"                 8000000 1600000   6400000  20% /var/lib/postgresql\n"
		free, used, total, err := parseDfOutput(output)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 8000000*1024 || used != 1600000*1024 || free != 6400000*1024 {
			t.Fatalf("unexpected values: free=%d used=%d total=%d", free, used, total)
		}
	})

	t.Run("fails on missing data line", func(t *testing.T) {
		if _, _, _, err := parseDfOutput("Filesystem 1-blocks Used Available Use% Mounted on\n"); err == nil {
			t.Fatal("expected error for header-only output")
		}
	})

	t.Run("fails on too few fields", func(t *testing.T) {
		if _, _, _, err := parseDfOutput("header\noverlay 123 456\n"); err == nil {
			t.Fatal("expected error for short data line")
		}
	})

	t.Run("fails on non-numeric fields", func(t *testing.T) {
		if _, _, _, err := parseDfOutput("header\noverlay abc def ghi 2% /exports\n"); err == nil {
			t.Fatal("expected error for non-numeric values")
		}
	})
}
