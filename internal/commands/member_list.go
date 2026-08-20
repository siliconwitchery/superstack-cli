package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

func MemberList(session Session, arguments []string) error {
	positionals, jsonOutput := takeJsonFlag(arguments)

	if len(positionals) != 1 {
		return errors.New("member list takes a fleet id")
	}

	fleetId, err := strconv.ParseInt(positionals[0], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	request, err := authenticatedRequest(session, http.MethodGet,
		"/fleets/"+strconv.FormatInt(fleetId, 10)+"/members", nil)

	if err != nil {
		return err
	}

	response, err := session.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your connection")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return serverError(response)
	}

	people := struct {
		Owner   string   `json:"owner"`
		Members []string `json:"members"`
	}{}

	err = json.NewDecoder(response.Body).Decode(&people)

	if err != nil {
		return err
	}

	if jsonOutput {
		return json.NewEncoder(session.Out).Encode(people)
	}

	emailWidth := max(len("EMAIL"), len(people.Owner))

	for _, email := range people.Members {
		emailWidth = max(emailWidth, len(email))
	}

	fmt.Fprintf(session.Out, "%-*s  %s\n", emailWidth, "EMAIL", "ROLE")

	fmt.Fprintf(session.Out, "%-*s  owner\n", emailWidth, people.Owner)

	for _, email := range people.Members {
		fmt.Fprintf(session.Out, "%-*s  member\n", emailWidth, email)
	}

	return nil
}
