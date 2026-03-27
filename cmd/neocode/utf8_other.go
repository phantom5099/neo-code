//go:build !windows

package main

// setUTF8Mode is a no-op on non-Windows systems because UTF-8 is already the default.
func setUTF8Mode() {}
