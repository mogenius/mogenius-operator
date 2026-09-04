package services

import (
	"strings"
	"testing"
)

func TestResolvePath(t *testing.T) {
	t.Run("rejects empty path", func(t *testing.T) {
		if _, err := resolvePath("/exports", ""); err == nil {
			t.Fatal("expected error for empty path")
		}
	})

	t.Run("rejects traversal attempts", func(t *testing.T) {
		for _, p := range []string{"/..", "/../etc", "/foo/../bar", "/foo/./bar", "./foo", "/~root", "~"} {
			if _, err := resolvePath("/exports", p); err == nil {
				t.Fatalf("expected error for path %q", p)
			}
		}
	})

	t.Run("root path resolves to mount root", func(t *testing.T) {
		got, err := resolvePath("/exports", "/")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/exports" {
			t.Fatalf("expected /exports, got %q", got)
		}
	})

	t.Run("nested path joins below mount root", func(t *testing.T) {
		got, err := resolvePath("/exports", "/foo/bar.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/exports/foo/bar.txt" {
			t.Fatalf("expected /exports/foo/bar.txt, got %q", got)
		}
	})

	t.Run("path without leading slash joins as well", func(t *testing.T) {
		got, err := resolvePath("/exports", "foo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/exports/foo" {
			t.Fatalf("expected /exports/foo, got %q", got)
		}
	})

	t.Run("trailing slash is preserved for legacy compatibility", func(t *testing.T) {
		got, err := resolvePath("/exports", "/foo/")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/exports/foo/" {
			t.Fatalf("expected /exports/foo/, got %q", got)
		}
	})

	t.Run("works against arbitrary mount roots", func(t *testing.T) {
		got, err := resolvePath("/var/lib/data", "/sub/dir")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/var/lib/data/sub/dir" {
			t.Fatalf("expected /var/lib/data/sub/dir, got %q", got)
		}
	})
}

func TestSearchFindArgs(t *testing.T) {
	args := searchFindArgs("/data", "err")
	joined := strings.Join(args, " ")

	t.Run("matches case-insensitively as substring", func(t *testing.T) {
		if !strings.Contains(joined, "-iname *err*") {
			t.Fatalf("expected -iname *err*, got %q", joined)
		}
	})

	t.Run("skips lost+found", func(t *testing.T) {
		if !strings.Contains(joined, "! -name lost+found") || !strings.Contains(joined, "! -path */lost+found/*") {
			t.Fatalf("expected lost+found exclusion, got %q", joined)
		}
	})

	t.Run("keeps the query as one argv element without a shell", func(t *testing.T) {
		args := searchFindArgs("/data", "a b; rm -rf /")
		if args[0] != "find" {
			t.Fatalf("search must exec find directly, got %v", args)
		}
		found := false
		for _, arg := range args {
			if arg == "*a b; rm -rf /*" {
				found = true
			}
			if arg == "sh" || arg == "/bin/sh" {
				t.Fatalf("search must not go through a shell: %v", args)
			}
		}
		if !found {
			t.Fatalf("query not passed verbatim: %v", args)
		}
	})

	t.Run("uses the shared stat format", func(t *testing.T) {
		if !strings.Contains(joined, "-exec stat -c "+statFormat+" {} ;") {
			t.Fatalf("expected stat exec, got %q", joined)
		}
	})
}
