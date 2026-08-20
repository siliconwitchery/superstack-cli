package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

func KeyCreate(session Session, arguments []string) error {
	if len(arguments) != 2 || arguments[1] == "" {
		return errors.New("key create takes a fleet id and a label, quoted if it has spaces")
	}

	fleetId, err := strconv.ParseInt(arguments[0], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	body, err := json.Marshal(map[string]string{"label": arguments[1]})

	if err != nil {
		return err
	}

	request, err := authenticatedRequest(session, http.MethodPost,
		"/fleets/"+strconv.FormatInt(fleetId, 10)+"/keys", bytes.NewReader(body))

	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := session.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your connection")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return serverError(response)
	}

	created := struct {
		Id  int64  `json:"id"`
		Key string `json:"key"`
	}{}

	err = json.NewDecoder(response.Body).Decode(&created)

	if err != nil {
		return err
	}

	fmt.Fprintf(session.Out, "Created key %d.\n\n  %s\n\nAnyone holding it can send data to the fleet, and you will not see it again.\n", created.Id, created.Key)

	return nil
}
