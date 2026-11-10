package tlsgateway

import (
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

// pathPrefixSegment is what a segment of a tunnel's path prefix may hold; the platform validates the same.
var pathPrefixSegment = regexp.MustCompile(`^[a-z0-9._~-]+$`)

// encodedSeparator finds an encoded "/", "\" or ".": a path that hides those cannot be read unambiguously.
var encodedSeparator = regexp.MustCompile(`(?i)%(2f|5c|2e)`)

// validPathPrefix reports whether prefix is the platform's canonical form: "/" followed by lower case segments, no
// dot segments, no trailing "/".
func validPathPrefix(prefix string) bool {
	if !strings.HasPrefix(prefix, "/") || len(prefix) > 255 {
		return false
	}
	for _, segment := range strings.Split(prefix[1:], "/") {
		if segment == "." || segment == ".." || !pathPrefixSegment.MatchString(segment) {
			return false
		}
	}
	return true
}

// normalizeRequestPath decodes a request path once and normalizes it ("." and ".." resolved, repeated "/" folded, a
// trailing "/" kept). It refuses what cannot be read unambiguously: an encoded "/", "\" or ".", a backslash, broken
// or double encoding, invalid UTF-8, control characters, or ".." above the root. Same rules as the platform's HTTP
// proxy (PortForwardTunnelUtils.normalizeRequestPath), with the same test table.
func normalizeRequestPath(raw string) (string, bool) {
	if !strings.HasPrefix(raw, "/") || strings.Contains(raw, `\`) || encodedSeparator.MatchString(raw) {
		return "", false
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil || !utf8.ValidString(decoded) || strings.ContainsAny(decoded, `%\`) || hasControl(decoded) {
		return "", false
	}
	segments := make([]string, 0)
	for _, segment := range strings.Split(decoded, "/") {
		switch segment {
		case "", ".":
			continue
		case "..":
			if len(segments) == 0 {
				return "", false
			}
			segments = segments[:len(segments)-1]
		default:
			segments = append(segments, segment)
		}
	}
	path := "/" + strings.Join(segments, "/")
	if strings.HasSuffix(decoded, "/") && len(segments) > 0 {
		path += "/"
	}
	return path, true
}

// pathMatchesPrefix reports whether a normalized path lies under prefix: the prefix itself or below it, never
// "/webhook/githubx" for "/webhook/github".
func pathMatchesPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
