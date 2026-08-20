package api

import (
	"encoding/json"
	"errors"
	"net/http"
)

type FleetEntry struct {
	Id    int64  `json:"id"`
	Name  string `json:"name"`
	Owner bool   `json:"owner"`
}

func FetchFleets(session Session) ([]FleetEntry, error) {
	request, err := AuthenticatedRequest(session, http.MethodGet, "/fleets", nil)

	if err != nil {
		return nil, err
	}

	response, err := session.Client.Do(request)

	if err != nil {
		return nil, errors.New("the server could not be reached, check your connection")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, ServerError(response)
	}

	fleets := []FleetEntry{}

	err = json.NewDecoder(response.Body).Decode(&fleets)

	if err != nil {
		return nil, err
	}

	return fleets, nil
}
