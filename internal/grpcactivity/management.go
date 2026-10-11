package grpcactivity

import (
	"net"
	"strconv"
	"strings"
)

// PortFromListenAddress extracts the smart HTTP management port. The listen
// host is intentionally ignored: the connection filter matches the actual
// private destination reported by sing-box, not the wildcard listen address.
func PortFromListenAddress(address string) uint16 {
	_, port, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return 0
	}
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return 0
	}
	return uint16(n)
}

// IsManagementDestination identifies requests to smart itself. Matching the
// private/local destination as well as the port prevents an unrelated public
// service using the same port from being discarded.
func IsManagementDestination(destination string, port uint16) bool {
	if port == 0 {
		return false
	}
	host, portText, err := net.SplitHostPort(strings.TrimSpace(destination))
	if err != nil {
		return false
	}
	n, err := strconv.ParseUint(portText, 10, 16)
	if err != nil || uint16(n) != port {
		return false
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast())
}
