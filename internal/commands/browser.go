package commands

import (
	"os/exec"
	"runtime"
)

func openBrowser(url string) {
	var command *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url)

	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)

	default:
		command = exec.Command("xdg-open", url)
	}

	_ = command.Start()
}
