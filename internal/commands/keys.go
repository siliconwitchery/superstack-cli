package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type keyEntry struct {
	Id     int64  `json:"id"`
	Fleet  int64  `json:"fleet"`
	Label  string `json:"label"`
	Suffix string `json:"suffix"`
}

func fetchKeys(session Session) ([]keyEntry, error) {
	request, err := authenticatedRequest(session, http.MethodGet, "/keys", nil)

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

	keys := []keyEntry{}

	err = json.NewDecoder(response.Body).Decode(&keys)

	if err != nil {
		return nil, err
	}

	return keys, nil
}
