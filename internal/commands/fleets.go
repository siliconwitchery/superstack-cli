package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type fleetEntry struct {
	Id    int64  `json:"id"`
	Name  string `json:"name"`
	Owner bool   `json:"owner"`
}

func fetchFleets() ([]fleetEntry, error) {
	request, err := authenticatedRequest(http.MethodGet, "/fleets", nil)

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

	fleets := []fleetEntry{}

	err = json.NewDecoder(response.Body).Decode(&fleets)

	if err != nil {
		return nil, err
	}

	return fleets, nil
}
