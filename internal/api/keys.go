package api

import (
	"errors"
	"net/http"
)

type KeyEntry struct {
	Id     int64  `json:"id"`
	Fleet  int64  `json:"fleet"`
	Label  string `json:"label"`
	Suffix string `json:"suffix"`
}

func FetchKeys(session Session) ([]KeyEntry, error) {
	request, err := AuthenticatedRequest(session, http.MethodGet, "/keys", nil)

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

	keys := []KeyEntry{}

	err = Decode(response, &keys)

	if err != nil {
		return nil, err
	}

	return keys, nil
}
