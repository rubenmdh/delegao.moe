package main

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func parseProxies(value string) ([]netip.Prefix, error) {
	var result []netip.Prefix
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	for _, part := range strings.Split(value, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES must contain comma-separated CIDR ranges")
		}
		result = append(result, prefix)
	}
	return result, nil
}
func (a *App) clientAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	current, err := netip.ParseAddr(host)
	if err != nil {
		return r.RemoteAddr
	}
	trusted := func(ip netip.Addr) bool {
		for _, prefix := range a.proxies {
			if prefix.Contains(ip.Unmap()) {
				return true
			}
		}
		return false
	}
	if !trusted(current) {
		return r.RemoteAddr
	}
	chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	if len(chain) > 20 {
		return r.RemoteAddr
	}
	for i := len(chain) - 1; i >= 0 && trusted(current); i-- {
		next, err := netip.ParseAddr(strings.TrimSpace(chain[i]))
		if err != nil {
			return r.RemoteAddr
		}
		current = next
	}
	return net.JoinHostPort(current.String(), "0")
}
