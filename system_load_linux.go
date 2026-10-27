//go:build linux

package main

import "os"

func systemLoad() string {
	// /proc is supplied by Linux; this needs no process, polling or dependency.
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return ""
	}
	return formatSystemLoad(string(data))
}
