//go:build !linux

package main

// An unavailable measurement is omitted rather than reported as zero.
func systemLoad() string { return "" }
