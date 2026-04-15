package apps

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// CheckAllowedHost returns nil if the URL's host matches the allowedHosts list.
// Rules:
//   - Wildcard prefix: "*.example.com" matches "foo.example.com" but not "example.com"
//   - Exact match: "localhost:9000" matches "localhost:9000"
//   - Port wildcard: "localhost:*" matches "localhost" with any port
//   - IP literals blocked by default unless explicitly listed
//   - Empty allowedHosts = no outbound HTTP allowed
func CheckAllowedHost(rawURL string, allowedHosts []string) error {
	if len(allowedHosts) == 0 {
		return fmt.Errorf("outbound HTTP blocked: app has no allowedHosts")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()

	// Block raw IP addresses unless explicitly listed.
	if net.ParseIP(host) != nil {
		for _, allowed := range allowedHosts {
			if strings.ToLower(allowed) == host || strings.ToLower(allowed) == host+":"+port {
				return nil
			}
		}
		return fmt.Errorf("IP address %s not in allowedHosts (add it explicitly to allow)", host)
	}

	hostWithPort := host
	if port != "" {
		hostWithPort = host + ":" + port
	}

	for _, pattern := range allowedHosts {
		pattern = strings.ToLower(pattern)
		if matchHostPattern(pattern, hostWithPort, host, port) {
			return nil
		}
	}

	return fmt.Errorf("host %q not in allowedHosts %v", hostWithPort, allowedHosts)
}

func matchHostPattern(pattern, hostWithPort, host, port string) bool {
	// Exact match with port.
	if pattern == hostWithPort {
		return true
	}
	// Exact match without port.
	if pattern == host {
		return true
	}

	// Port wildcard: "localhost:*"
	if strings.HasSuffix(pattern, ":*") {
		patternHost := strings.TrimSuffix(pattern, ":*")
		if patternHost == host {
			return true
		}
	}

	// Wildcard prefix: "*.example.com"
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".example.com"
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}

	return false
}
