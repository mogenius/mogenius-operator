package kubernetes

import (
	"testing"
)

func TestParseDfOutput(t *testing.T) {
	t.Run("parses regular df -B1 output", func(t *testing.T) {
		output := "Filesystem     1-blocks      Used Available Use% Mounted on\n" +
			"overlay        5368709120  102400  5266309120   2% /exports\n"
		free, used, total, err := parseDfOutput(output)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 5368709120 || used != 102400 || free != 5266309120 {
			t.Fatalf("unexpected values: free=%d used=%d total=%d", free, used, total)
		}
	})

	t.Run("uses the last line of multi-line output", func(t *testing.T) {
		output := "Filesystem     1-blocks      Used Available Use% Mounted on\n" +
			"tmpfs          999  1  998   1% /other\n" +
			"/dev/sda1      1000  200  800   20% /data\n"
		free, used, total, err := parseDfOutput(output)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if total != 1000 || used != 200 || free != 800 {
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
