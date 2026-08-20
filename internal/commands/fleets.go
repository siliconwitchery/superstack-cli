package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type fleetEntry struct {
	Id    int64  `json:"id"`
	Name  string `json:"name"`
	Owner bool   `json:"owner"`
}

func fetchFleets(session Session) ([]fleetEntry, error) {
	request, err := authenticatedRequest(session, http.MethodGet, "/fleets", nil)

	if err != nil {
		return nil, err
	}

	response, err := session.Client.Do(request)

	if err != nil {
		return nil, fmt.Errorf("the server could not be reached: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, serverError(response)
	}

	fleets := []fleetEntry{}

	err = json.NewDecoder(response.Body).Decode(&fleets)

	if err != nil {
		return nil, err
	}

	return fleets, nil
}
