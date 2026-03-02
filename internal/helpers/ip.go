package helpers

import (
	"fmt"
	"net"
	"strings"
)

// ExtractIPFromAddr extracts the IP address from a net.Addr
func ExtractIPFromAddr(addr net.Addr) (string, error) {
	if addr == nil {
		return "", fmt.Errorf("nil address")
	}

	// Handle TCP addresses
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		return tcpAddr.IP.String(), nil
	}

	// Handle UDP addresses
	if udpAddr, ok := addr.(*net.UDPAddr); ok {
		return udpAddr.IP.String(), nil
	}

	// Fallback: parse from string representation
	addrStr := addr.String()
	host, _, err := net.SplitHostPort(addrStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse address: %w", err)
	}

	return host, nil
}

// ExtractPortFromAddr extracts the port from a net.Addr
func ExtractPortFromAddr(addr net.Addr) (int, error) {
	if addr == nil {
		return 0, fmt.Errorf("nil address")
	}

	// Handle TCP addresses
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		return tcpAddr.Port, nil
	}

	// Handle UDP addresses
	if udpAddr, ok := addr.(*net.UDPAddr); ok {
		return udpAddr.Port, nil
	}

	// Fallback: parse from string representation
	addrStr := addr.String()
	_, portStr, err := net.SplitHostPort(addrStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse address: %w", err)
	}

	port := 0
	fmt.Sscanf(portStr, "%d", &port)
	return port, nil
}

// IsIPv4 checks if an IP address is IPv4
func IsIPv4(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return ip.To4() != nil
}

// IsIPv6 checks if an IP address is IPv6
func IsIPv6(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return ip.To4() == nil && ip.To16() != nil
}

// NormalizeIP normalizes an IP address string
func NormalizeIP(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ipStr
	}
	return ip.String()
}

// IsLocalhost checks if an IP is localhost
func IsLocalhost(ipStr string) bool {
	normalized := NormalizeIP(ipStr)
	return normalized == "127.0.0.1" || normalized == "::1" || strings.HasPrefix(normalized, "127.")
}

// IsPrivateIP checks if an IP is in a private range
func IsPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// Check if IPv4
	if ip.To4() != nil {
		// Private IPv4 ranges: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
		private := []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"127.0.0.0/8",
		}

		for _, cidr := range private {
			_, ipnet, _ := net.ParseCIDR(cidr)
			if ipnet.Contains(ip) {
				return true
			}
		}
	} else {
		// Check if IPv6 local
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return true
		}
	}

	return false
}
