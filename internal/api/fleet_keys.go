package api

import (
	"errors"
	"net/http"
)

type FleetKeyEntry struct {
	Id     int64  `json:"id"`
	Fleet  int64  `json:"fleet"`
	Label  string `json:"label"`
	Suffix string `json:"suffix"`
}

func FetchFleetKeys(invocation Invocation) ([]FleetKeyEntry, error) {
	request, err := AuthenticatedRequest(invocation, http.MethodGet, "/keys", nil)

	if err != nil {
		return nil, err
	}

	response, err := invocation.Client.Do(request)

	if err != nil {
		return nil, errors.New("the server could not be reached, check your internet access")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, ServerError(response)
	}

	fleetKeys := []FleetKeyEntry{}

	err = Decode(response, &fleetKeys)

	if err != nil {
		return nil, err
	}

	return fleetKeys, nil
}
