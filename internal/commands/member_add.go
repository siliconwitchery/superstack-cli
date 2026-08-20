package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

func MemberAdd(session Session, arguments []string) error {

	if len(arguments) != 2 || arguments[0] == "" {
		return errors.New("member add takes an email address and a fleet id")
	}

	email := arguments[0]

	fleetId, err := strconv.ParseInt(arguments[1], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	body, err := json.Marshal(map[string]string{"email": email})

	if err != nil {
		return err
	}

	request, err := authenticatedRequest(session, http.MethodPost,
		"/fleets/"+strconv.FormatInt(fleetId, 10)+"/members", bytes.NewReader(body))

	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := session.Client.Do(request)

	if err != nil {
		return fmt.Errorf("the server could not be reached: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return serverError(response)
	}

	fmt.Fprintf(session.Out, "Gave %s access.\n", email)

	return nil
}
