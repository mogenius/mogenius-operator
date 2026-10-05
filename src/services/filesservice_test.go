package services

import (
	"mogenius-operator/src/dtos"
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
	args := searchFindArgs("/data", "err", false)
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

	t.Run("keeps the query as one argv element outside the stat script", func(t *testing.T) {
		args := searchFindArgs("/data", "a b; rm -rf /", false)
		if args[0] != "find" {
			t.Fatalf("search must exec find directly, got %v", args)
		}
		found := false
		for _, arg := range args {
			if arg == "*a b; rm -rf /*" {
				found = true
			}
			// the only shell is the fixed stat script after -exec; the query must never be part of it
			if strings.Contains(arg, "stat -c") && strings.Contains(arg, "rm -rf") {
				t.Fatalf("query leaked into the shell script: %v", args)
			}
		}
		if !found {
			t.Fatalf("query not passed verbatim: %v", args)
		}
	})

	t.Run("uses the shared stat format", func(t *testing.T) {
		if !strings.Contains(joined, "-exec sh -c "+statRecordScript+" sh {} +") {
			t.Fatalf("expected stat exec, got %q", joined)
		}
	})
}

// Daytona's searchFiles passes shell globs; with glob set the query is the
// whole -name test, so `*.py` does not also match `x.pyc`.
func TestSearchFindArgsGlob(t *testing.T) {
	joined := strings.Join(searchFindArgs("/data", "*.py", true), " ")
	if !strings.Contains(joined, "-name *.py") || strings.Contains(joined, "-iname") {
		t.Fatalf("expected an exact -name glob test, got %q", joined)
	}
}

func TestSplitStatRecordsKeepsNewlineNames(t *testing.T) {
	output := "/data/new\nline.txt\tregular file\t5\t1000\t1000\t644\t1\n\x00/data/plain.txt\tregular file\t5\t1000\t1000\t644\t1\n\x00"
	records := splitStatRecords(output)
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d: %q", len(records), records)
	}
	item, err := parseStatLine("/data", records[0])
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != "new\nline.txt" {
		t.Errorf("name = %q, want the newline kept", item.Name)
	}
	if item.RelativePath != "new\nline.txt" {
		t.Errorf("relative path = %q", item.RelativePath)
	}
}

func TestSplitStatRecordsAcceptsPlainLines(t *testing.T) {
	records := splitStatRecords("a\tb\n\nc\td\n")
	if len(records) != 2 || records[0] != "a\tb" || records[1] != "c\td" {
		t.Errorf("unexpected records %q", records)
	}
}

func TestHeaderFilename(t *testing.T) {
	cases := map[string]string{
		`plain.txt`:         `plain.txt`,
		`it's "quoted".txt`: `it's \"quoted\".txt`,
		"new\nline.txt":     `new_line.txt`,
		`back\slash`:        `back\\slash`,
	}
	for in, want := range cases {
		if got := headerFilename(in); got != want {
			t.Errorf("headerFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDownloadNameAndType(t *testing.T) {
	cases := []struct {
		name        string
		info        dtos.PersistentFileDto
		wantName    string
		wantContent string
	}{
		{"file keeps its sniffed type", dtos.PersistentFileDto{Name: "notes", Type: "file", ContentType: "text/plain"}, "notes", "text/plain"},
		{"file without type is an octet stream", dtos.PersistentFileDto{Name: "blob.bin", Type: "file"}, "blob.bin", "application/octet-stream"},
		{"directory becomes a tar.gz", dtos.PersistentFileDto{Name: "src", Type: "directory", ContentType: "inode/directory"}, "src.tar.gz", "application/gzip"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			name, contentType := downloadNameAndType(c.info)
			if name != c.wantName || contentType != c.wantContent {
				t.Fatalf("downloadNameAndType = %q, %q; want %q, %q", name, contentType, c.wantName, c.wantContent)
			}
		})
	}
}
