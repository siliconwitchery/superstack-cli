package commands

import (
	"os/exec"
	"runtime"
)

// A variable so tests can swap in a recorder instead of reaching a real
// browser. Opening is best effort: the link is already on screen.
var openBrowser = func(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start()

	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()

	default:
		exec.Command("xdg-open", url).Start()
	}
}
