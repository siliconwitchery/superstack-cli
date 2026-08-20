package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func FleetRename(session Session, arguments []string) error {

	if len(arguments) != 2 {
		return errors.New("fleet rename takes a fleet id and a name, quoted if it has spaces")
	}

	fleetId, err := strconv.ParseInt(arguments[0], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	name := strings.TrimSpace(arguments[1])

	if name == "" {
		return errors.New("fleet rename takes a fleet id and a name, quoted if it has spaces")
	}

	body, err := json.Marshal(map[string]string{"name": name})

	if err != nil {
		return err
	}

	request, err := authenticatedRequest(session, http.MethodPatch,
		"/fleets/"+strconv.FormatInt(fleetId, 10), bytes.NewReader(body))

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

	fmt.Fprintf(session.Out, "Renamed the fleet to %q.\n", name)

	return nil
}
