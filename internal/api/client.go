package api

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func Request(session Session, method string, path string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequest(method, strings.TrimSuffix(session.Base, "/")+path, body)

	if err != nil {
		return nil, err
	}

	request.Header.Set("User-Agent", "superstack/"+session.Version)

	return request, nil
}

func AuthenticatedRequest(session Session, method string, path string, body io.Reader) (*http.Request, error) {
	storedKeyPath, err := KeyPath()

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

	request, err := Request(session, method, path, body)

	if err != nil {
		return nil, err
	}

	request.Header.Set("Authorization", "Bearer "+key)

	return request, nil
}

func ServerError(response *http.Response) error {
	message, err := io.ReadAll(io.LimitReader(response.Body, 4096))

	detail := strings.TrimSpace(string(message))

	if err != nil || detail == "" {
		return errors.New("that did not go through, try again in a moment")
	}

	return errors.New(detail)
}

func KeyPath() (string, error) {
	if runtime.GOOS == "linux" {
		stateHome := os.Getenv("XDG_STATE_HOME")

		// The xdg base directory spec says to ignore a relative XDG_STATE_HOME.
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
