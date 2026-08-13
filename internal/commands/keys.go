package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type keyEntry struct {
	Id     int64  `json:"id"`
	Fleet  int64  `json:"fleet"`
	Label  string `json:"label"`
	Suffix string `json:"suffix"`
}

func fetchKeys() ([]keyEntry, error) {
	request, err := authenticatedRequest(http.MethodGet, "/keys", nil)

	if err != nil {
		return nil, err
	}

	response, err := apiClient.Do(request)

	if err != nil {
		return nil, fmt.Errorf("the server could not be reached: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))

		return nil, fmt.Errorf("the server said: %s", strings.TrimSpace(string(message)))
	}

	keys := []keyEntry{}

	err = json.NewDecoder(response.Body).Decode(&keys)

	if err != nil {
		return nil, err
	}

	return keys, nil
}
