package api

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func Request(invocation Invocation, method string, path string, reader io.Reader) (*http.Request, error) {
	request, err := http.NewRequest(method, strings.TrimSuffix(invocation.Base, "/")+path, reader)

	if err != nil {
		return nil, err
	}

	request.Header.Set("User-Agent", "superstack/"+invocation.Version)

	return request, nil
}

func AuthenticatedRequest(invocation Invocation, method string, path string, reader io.Reader) (*http.Request, error) {
	loginKeyPath, err := LoginKeyPath()

	if err != nil {
		return nil, err
	}

	loginKeyBytes, err := os.ReadFile(loginKeyPath)

	if errors.Is(err, fs.ErrNotExist) {
		return nil, errors.New("you are not logged in, run login first")
	}

	if err != nil {
		return nil, errors.New("the login stored on this computer could not be read")
	}

	loginKey := strings.TrimSpace(string(loginKeyBytes))

	if loginKey == "" {
		return nil, errors.New("you are not logged in, run login first")
	}

	request, err := Request(invocation, method, path, reader)

	if err != nil {
		return nil, err
	}

	request.Header.Set("Authorization", "Bearer "+loginKey)

	return request, nil
}

func ServerError(response *http.Response) error {
	message, err := io.ReadAll(io.LimitReader(response.Body, 4096))

	detail := strings.TrimSpace(string(message))

	if err != nil || detail == "" {
		return errors.New("that did not go through, try again in a moment")
	}

	return errors.New(Printable(detail))
}

func Decode(response *http.Response, value any) error {
	// 32 MiB is past every capped list and is the ceiling on the uncapped ones.
	err := json.NewDecoder(io.LimitReader(response.Body, 32<<20)).Decode(value)

	if err != nil {
		return errors.New("the server's answer could not be read, try again in a moment")
	}

	return nil
}

func LoginKeyPath() (string, error) {
	if runtime.GOOS == "linux" {
		stateHome := os.Getenv("XDG_STATE_HOME")

		// The xdg base directory spec says to ignore a relative XDG_STATE_HOME.
		if !filepath.IsAbs(stateHome) {
			home, err := os.UserHomeDir()

			if err != nil {
				return "", errors.New("your home folder could not be found, so the login cannot be read or saved")
			}

			stateHome = filepath.Join(home, ".local", "state")
		}

		return filepath.Join(stateHome, "superstack", "key"), nil
	}

	configDirectory, err := os.UserConfigDir()

	if err != nil {
		return "", errors.New("your settings folder could not be found, so the login cannot be read or saved")
	}

	return filepath.Join(configDirectory, "superstack", "key"), nil
}
