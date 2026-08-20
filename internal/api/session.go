package api

import (
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

const DefaultBase = "https://supernext.siliconwitchery.com"
const defaultGithubBase = "https://github.com"
const defaultGitlabBase = "https://gitlab.com"

type Session struct {
	Base        string
	GithubBase  string
	GitlabBase  string
	Version     string
	Client      *http.Client
	In          io.Reader
	Out         io.Writer
	OpenBrowser func(url string)
}

func NewSession(base string, version string, in io.Reader, out io.Writer) Session {
	return Session{
		Base:        base,
		GithubBase:  defaultGithubBase,
		GitlabBase:  defaultGitlabBase,
		Version:     version,
		Client:      &http.Client{Timeout: 30 * time.Second},
		In:          in,
		Out:         out,
		OpenBrowser: openBrowser,
	}
}

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
