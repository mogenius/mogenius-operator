package services

import (
	"strings"
	"testing"
)

// A pod's own filesystem has mount root "/": paths must come out as plain
// absolute paths, and the escape check must still hold.
func TestResolvePathOnRootMount(t *testing.T) {
	cases := map[string]string{
		"/":                 "/",
		"/home/coder":       "/home/coder",
		"home/coder":        "/home/coder",
		"/home/coder/a.txt": "/home/coder/a.txt",
		"/with space/x":     "/with space/x",
	}
	for in, want := range cases {
		got, err := resolvePath("/", in)
		if err != nil {
			t.Errorf("resolvePath(/, %q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("resolvePath(/, %q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"/..", "/etc/../root", "~/x", ""} {
		if _, err := resolvePath("/", bad); err == nil {
			t.Errorf("resolvePath(/, %q) accepted", bad)
		}
	}
}

// The debug container sees the target's files under /proc/1/root; a request
// path must map there and back without the prefix leaking into results.
func TestResolvePathUnderDebugContainerRoot(t *testing.T) {
	got, err := resolvePath(debugContainerRootPath, "/home/coder/a.txt")
	if err != nil || got != "/proc/1/root/home/coder/a.txt" {
		t.Fatalf("got %q, %v", got, err)
	}
	if back := requestPathOf(debugContainerRootPath, got); back != "/home/coder/a.txt" {
		t.Errorf("requestPathOf = %q", back)
	}
	if back := requestPathOf(debugContainerRootPath, "/proc/1/root"); back != "/" {
		t.Errorf("requestPathOf(root) = %q", back)
	}
	if back := requestPathOf("/", "/home/coder"); back != "/home/coder" {
		t.Errorf("requestPathOf on / changed the path: %q", back)
	}
}

func TestNormalizeMode(t *testing.T) {
	for in, want := range map[string]string{"755": "0755", "0644": "0644", "7": "0007", "u+x": "u+x", "go-w,u=rwx": "go-w,u=rwx"} {
		got, err := normalizeMode(in)
		if err != nil || got != want {
			t.Errorf("normalizeMode(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "999", "rwx", "0o755", "u+q", "755; rm -rf /"} {
		if _, err := normalizeMode(bad); err == nil {
			t.Errorf("normalizeMode(%q) accepted", bad)
		}
	}
}

// Daytona hands owner and group as names; numeric ids and root (0) stay valid,
// and nothing that could be read as an option or shell syntax passes.
func TestValidateOwnerPart(t *testing.T) {
	for _, ok := range []string{"0", "1000", "coder", "www-data", "_apt", "svc$", "a.b"} {
		if err := validateOwnerPart(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "-R", "root:root", "a b", "4294967296", "Admin", "$(id)"} {
		if err := validateOwnerPart(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestFindGrepArgs(t *testing.T) {
	args := findGrepArgs("/home/coder", "-dash")
	joined := strings.Join(args, " ")
	if !strings.HasPrefix(joined, "grep -rnI -e -dash -- /home/coder") {
		t.Fatalf("unexpected argv %q", joined)
	}
}

func TestParseGrepOutput(t *testing.T) {
	output := strings.Join([]string{
		"/proc/1/root/home/coder/a.txt:3:hello world",
		"/proc/1/root/home/coder/b.txt:10:a:b:c with colons",
		"garbage line without the shape",
		"/proc/1/root/home/coder/c.txt:1:third",
	}, "\n") + "\n"

	matches, truncated := parseGrepOutput(output, debugContainerRootPath, 10)
	if truncated {
		t.Error("truncated set although under the cap")
	}
	if len(matches) != 3 {
		t.Fatalf("got %d matches: %+v", len(matches), matches)
	}
	if matches[0].File != "/home/coder/a.txt" || matches[0].Line != 3 || matches[0].Content != "hello world" {
		t.Errorf("first match = %+v", matches[0])
	}
	// the content may itself contain colons; only the first two separators are structural
	if matches[1].Content != "a:b:c with colons" {
		t.Errorf("second match content = %q", matches[1].Content)
	}

	capped, truncated := parseGrepOutput(output, debugContainerRootPath, 2)
	if len(capped) != 2 || !truncated {
		t.Errorf("cap not applied: %d matches, truncated=%v", len(capped), truncated)
	}

	if matches, _ := parseGrepOutput("", "/", 10); len(matches) != 0 {
		t.Errorf("empty output produced matches")
	}
}
