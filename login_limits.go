package main

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
)

// Optional private build default, supplied with -ldflags -X. Public builds leave
// this empty. Never put a deployment's personal addresses in versioned sources.
var buildLoginRateLimitWhitelist string

func configuredLoginRateLimitWhitelist() (map[netip.Addr]struct{}, error) {
	value, present := os.LookupEnv("LOGIN_RATE_LIMIT_WHITELIST")
	if !present {
		value = buildLoginRateLimitWhitelist
	}
	return parseLoginRateLimitWhitelist(value)
}

func parseLoginRateLimitWhitelist(value string) (map[netip.Addr]struct{}, error) {
	allowed := make(map[netip.Addr]struct{})
	if strings.TrimSpace(value) == "" {
		return allowed, nil
	}
	for _, item := range strings.Split(value, ",") {
		address, err := netip.ParseAddr(strings.TrimSpace(item))
		if err != nil || address.Zone() != "" {
			return nil, errors.New("LOGIN_RATE_LIMIT_WHITELIST must contain comma-separated IPv4 or IPv6 addresses")
		}
		allowed[address.Unmap()] = struct{}{}
	}
	return allowed, nil
}

func (a *App) allowLogin(r *http.Request) bool {
	// handler has already resolved RemoteAddr through the trusted proxy chain.
	// Never read X-Forwarded-For directly here: clients can forge that header.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if address, err := netip.ParseAddr(host); err == nil {
		if _, allowed := a.loginRateLimitWhitelist[address.Unmap()]; allowed {
			return true
		}
	}
	return a.security.allow(r, "login", 10)
}
