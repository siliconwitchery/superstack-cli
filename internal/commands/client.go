package commands

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Deliberately absent from the help: development needs to point a command at
// another server, users never do.
func TakeServerFlag(arguments []string) ([]string, string, error) {
	remaining := []string{}

	base := defaultApiBase

	for index := 0; index < len(arguments); index++ {
		switch {
		case arguments[index] == "--server":
			if index+1 == len(arguments) || arguments[index+1] == "" {
				return nil, "", errors.New("--server needs a url")
			}

			index++

			base = arguments[index]

		case strings.HasPrefix(arguments[index], "--server="):
			base = strings.TrimPrefix(arguments[index], "--server=")

			if base == "" {
				return nil, "", errors.New("--server needs a url")
			}

		default:
			remaining = append(remaining, arguments[index])
		}
	}

	return remaining, strings.TrimSuffix(base, "/"), nil
}

func CheckServer(session Session) error {
	request, err := apiRequest(session, http.MethodGet, "/", nil)

	if err != nil {
		return err
	}

	response, err := session.Client.Do(request)

	if err != nil {
		return fmt.Errorf("the server at %s cannot be reached", strings.TrimSuffix(request.URL.String(), "/"))
	}

	response.Body.Close()

	return nil
}

func apiRequest(session Session, method string, path string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequest(method, strings.TrimSuffix(session.Base, "/")+path, body)

	if err != nil {
		return nil, err
	}

	request.Header.Set("User-Agent", "superstack/"+session.Version)

	return request, nil
}

func authenticatedRequest(session Session, method string, path string, body io.Reader) (*http.Request, error) {
	storedKeyPath, err := keyPath()

	if err != nil {
		return nil, err
	}

	keyBytes, err := os.ReadFile(storedKeyPath)

	if errors.Is(err, fs.ErrNotExist) {
		return nil, errors.New("you are not logged in, run login first")
	}

	if err != nil {
		return nil, err
	}

	key := strings.TrimSpace(string(keyBytes))

	if key == "" {
		return nil, errors.New("you are not logged in, run login first")
	}

	request, err := apiRequest(session, method, path, body)

	if err != nil {
		return nil, err
	}

	request.Header.Set("Authorization", "Bearer "+key)

	return request, nil
}

func serverError(response *http.Response) error {
	message, err := io.ReadAll(io.LimitReader(response.Body, 4096))

	detail := strings.TrimSpace(string(message))

	if err != nil || detail == "" {
		detail = response.Status
	}

	return fmt.Errorf("the server said: %s", detail)
}

func takeJsonFlag(arguments []string) ([]string, bool) {
	positionals := []string{}
	jsonOutput := false

	for _, argument := range arguments {
		if argument == "--json" {
			jsonOutput = true
			continue
		}

		positionals = append(positionals, argument)
	}

	return positionals, jsonOutput
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
