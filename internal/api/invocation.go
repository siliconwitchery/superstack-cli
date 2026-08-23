package api

import (
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"
)

const DefaultBase = "https://supernext.siliconwitchery.com"

type Invocation struct {
	Base        string
	GithubBase  string
	GitlabBase  string
	Version     string
	Client      *http.Client
	In          io.Reader
	Out         io.Writer
	OpenBrowser func(address string)
}

func NewInvocation(base string, version string, in io.Reader, out io.Writer) Invocation {
	return Invocation{
		Base:        base,
		GithubBase:  "https://github.com",
		GitlabBase:  "https://gitlab.com",
		Version:     version,
		Client:      &http.Client{Timeout: 30 * time.Second},
		In:          in,
		Out:         out,
		OpenBrowser: openBrowser,
	}
}

func openBrowser(address string) {
	parsed, err := url.Parse(address)

	if err != nil {
		return
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return
	}

	var command *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", address)

	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", address)

	default:
		command = exec.Command("xdg-open", address)
	}

	_ = command.Start()
}
