//go:build windows

package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

const utf8CodePage = 65001

func setUTF8Mode() {
	if err := windows.SetConsoleOutputCP(utf8CodePage); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to enable UTF-8 console output: %v\n", err)
	}
	if err := windows.SetConsoleCP(utf8CodePage); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to enable UTF-8 console input: %v\n", err)
	}
}
