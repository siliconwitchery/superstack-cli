package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

func FleetTransfer(session Session, arguments []string) error {
	if len(arguments) != 2 || arguments[1] == "" {
		return errors.New("fleet transfer takes a fleet id and an email address")
	}

	fleetId, err := strconv.ParseInt(arguments[0], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	email := arguments[1]

	body, err := json.Marshal(map[string]string{"email": email})

	if err != nil {
		return err
	}

	request, err := authenticatedRequest(session, http.MethodPost,
		"/fleets/"+strconv.FormatInt(fleetId, 10)+"/owner", bytes.NewReader(body))

	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := session.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your connection")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return serverError(response)
	}

	fmt.Fprintf(session.Out, "Transferred the fleet to %s.\n", email)

	return nil
}
