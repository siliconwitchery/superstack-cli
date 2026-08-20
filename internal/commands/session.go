package commands

import (
	"io"
	"net/http"
	"time"
)

const defaultApiBase = "https://supernext.siliconwitchery.com"
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
