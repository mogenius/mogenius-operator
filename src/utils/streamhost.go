package utils

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// CheckPlatformStreamURL decides whether the operator may dial a stream
// gateway URL that arrived in a request payload (terminal, exec, logs,
// events, port-forward, file download, live stats, AI chat). Every such dial
// carries the cluster credentials (x-authorization: MO_API_KEY), so an
// arbitrary host would leak them.
//
// Allowed are the hosts of MO_API_SERVER and MO_EVENT_SERVER, every host
// under their parent domain (k8s-ws.mogenius.com allows *.mogenius.com, which
// covers the separate k8s-cmd-stream host the platform uses) and the entries
// of MO_STREAM_ALLOWED_HOSTS. When the platform connection uses wss, plain ws
// is refused so the credentials cannot be downgraded to cleartext.
func CheckPlatformStreamURL(u url.URL) error {
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return fmt.Errorf("stream url has no host")
	}

	var platformHosts []string
	requireTLS := false
	for _, key := range []string{"MO_API_SERVER", "MO_EVENT_SERVER"} {
		raw, err := config.TryGet(key)
		if err != nil {
			continue
		}
		platform, err := url.Parse(raw)
		if err != nil || platform.Hostname() == "" {
			continue
		}
		platformHosts = append(platformHosts, strings.ToLower(platform.Hostname()))
		if platform.Scheme == "wss" || platform.Scheme == "https" {
			requireTLS = true
		}
	}

	switch u.Scheme {
	case "wss":
	case "ws":
		if requireTLS {
			return fmt.Errorf("stream url %q uses unencrypted ws while the platform connection uses tls", u.Host)
		}
	default:
		return fmt.Errorf("stream url %q has unsupported scheme %q", u.Host, u.Scheme)
	}

	for _, platformHost := range platformHosts {
		if host == platformHost {
			return nil
		}
		if parent := parentDomain(platformHost); parent != "" && strings.HasSuffix(host, "."+parent) {
			return nil
		}
	}
	extra, _ := config.TryGet("MO_STREAM_ALLOWED_HOSTS")
	for entry := range strings.SplitSeq(extra, ",") {
		if streamHostMatches(host, strings.ToLower(strings.TrimSpace(entry))) {
			return nil
		}
	}
	return fmt.Errorf("stream host %q is not a platform host (allowed: %s and their subdomains, plus MO_STREAM_ALLOWED_HOSTS)", host, strings.Join(platformHosts, ", "))
}

// parentDomain strips the first label of a hostname: k8s-ws.mogenius.com →
// mogenius.com. It returns "" for IPs, single-label names (localhost) and
// when the parent would be a public suffix (mogenius.co.uk must not allow
// every *.co.uk).
func parentDomain(host string) string {
	if net.ParseIP(host) != nil {
		return ""
	}
	_, parent, ok := strings.Cut(host, ".")
	if !ok {
		return ""
	}
	registrable, err := publicsuffix.EffectiveTLDPlusOne(parent)
	if err != nil || (parent != registrable && !strings.HasSuffix(parent, "."+registrable)) {
		return ""
	}
	return parent
}

// streamHostMatches matches a MO_STREAM_ALLOWED_HOSTS entry: an exact host,
// or "*.example.com" for every subdomain of example.com.
func streamHostMatches(host, entry string) bool {
	if entry == "" {
		return false
	}
	if suffix, ok := strings.CutPrefix(entry, "*."); ok {
		return strings.HasSuffix(host, "."+suffix)
	}
	return host == entry
}
