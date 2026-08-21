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

type Session struct {
	Base        string
	GithubBase  string
	GitlabBase  string
	Version     string
	Client      *http.Client
	In          io.Reader
	Out         io.Writer
	OpenBrowser func(link string)
}

func NewSession(base string, version string, in io.Reader, out io.Writer) Session {
	return Session{
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

func openBrowser(link string) {
	address, err := url.Parse(link)

	if err != nil {
		return
	}

	if address.Scheme != "http" && address.Scheme != "https" {
		return
	}

	var command *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", link)

	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", link)

	default:
		command = exec.Command("xdg-open", link)
	}

	_ = command.Start()
}
