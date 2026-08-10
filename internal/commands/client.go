package commands

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const defaultApiBase = "https://supernext.siliconwitchery.com"

var CliVersion = "unknown"

var chosenApiBase = ""

var apiClient = &http.Client{Timeout: 30 * time.Second}

// Deliberately absent from the help: development needs to point a command at
// another server, users never do.
func TakeServerFlag(arguments []string) ([]string, error) {
	remaining := []string{}

	server := ""

	for index := 0; index < len(arguments); index++ {
		switch {
		case arguments[index] == "--server":
			if index+1 == len(arguments) || arguments[index+1] == "" {
				return nil, errors.New("--server needs a url")
			}

			index++

			server = arguments[index]

		case strings.HasPrefix(arguments[index], "--server="):
			server = strings.TrimPrefix(arguments[index], "--server=")

			if server == "" {
				return nil, errors.New("--server needs a url")
			}

		default:
			remaining = append(remaining, arguments[index])
		}
	}

	chosenApiBase = strings.TrimSuffix(server, "/")

	return remaining, nil
}

func CheckServer() error {
	request, err := apiRequest(http.MethodGet, "/", nil)

	if err != nil {
		return err
	}

	response, err := apiClient.Do(request)

	if err != nil {
		return fmt.Errorf("the server at %s cannot be reached", strings.TrimSuffix(request.URL.String(), "/"))
	}

	response.Body.Close()

	return nil
}

func apiRequest(method string, path string, body io.Reader) (*http.Request, error) {
	base := chosenApiBase

	if base == "" {
		base = os.Getenv("SUPERSTACK_API")
	}

	if base == "" {
		base = defaultApiBase
	}

	request, err := http.NewRequest(method, strings.TrimSuffix(base, "/")+path, body)

	if err != nil {
		return nil, err
	}

	request.Header.Set("User-Agent", "superstack/"+CliVersion)

	return request, nil
}

func keyPath() (string, error) {
	// The key is state, not configuration: linux dotfile repos routinely
	// publish all of ~/.config, so the key must never live there. The mac
	// and windows config directories are not published like that.
	if runtime.GOOS == "linux" {
		stateHome := os.Getenv("XDG_STATE_HOME")

		// The spec says to ignore a relative value, and honoring one could
		// drop the key inside a repository the user later pushes.
		if !filepath.IsAbs(stateHome) {
			home, err := os.UserHomeDir()

			if err != nil {
				return "", err
			}

			stateHome = filepath.Join(home, ".local", "state")
		}

		return filepath.Join(stateHome, "superstack", "key"), nil
	}

	configDirectory, err := os.UserConfigDir()

	if err != nil {
		return "", err
	}

	return filepath.Join(configDirectory, "superstack", "key"), nil
}
