package commands

import (
	"encoding/json"
	"errors"
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
		return nil, errors.New("the server could not be reached, check your connection")
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
