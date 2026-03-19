//go:build windows

package main

import "swingwingo/ui"

// DPI awareness is declared via app.manifest embedded in app.syso at build time.
// This is more reliable than calling SetProcessDpiAwarenessContext() at runtime
// because the manifest takes effect before any window is created.

func main() {
	ui.Run()
}
