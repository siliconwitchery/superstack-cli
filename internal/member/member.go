package member

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/siliconwitchery/superstack-cli/internal/api"
)

func Add(session api.Session, arguments []string) error {
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

	request, err := api.AuthenticatedRequest(session, http.MethodPost,
		"/fleets/"+strconv.FormatInt(fleetId, 10)+"/members", bytes.NewReader(body))

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
		return api.ServerError(response)
	}

	fmt.Fprintf(session.Out, "Gave %s access to fleet %d.\n", email, fleetId)

	return nil
}

func List(session api.Session, arguments []string) error {
	positionals, jsonOutput := api.TakeJsonFlag(arguments)

	if len(positionals) != 1 {
		return errors.New("member list takes a fleet id")
	}

	fleetId, err := strconv.ParseInt(positionals[0], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	request, err := api.AuthenticatedRequest(session, http.MethodGet,
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
		return api.ServerError(response)
	}

	people := struct {
		Owner   string   `json:"owner"`
		Members []string `json:"members"`
	}{}

	err = api.Decode(response, &people)

	if err != nil {
		return err
	}

	if jsonOutput {
		err = json.NewEncoder(session.Out).Encode(people)

		return err
	}

	owner := api.Printable(people.Owner)
	members := make([]string, len(people.Members))
	emailWidth := max(len("EMAIL"), len(owner))

	for index, email := range people.Members {
		members[index] = api.Printable(email)
		emailWidth = max(emailWidth, len(members[index]))
	}

	fmt.Fprintf(session.Out, "%-*s  %s\n", emailWidth, "EMAIL", "ROLE")

	fmt.Fprintf(session.Out, "%-*s  owner\n", emailWidth, owner)

	for _, email := range members {
		fmt.Fprintf(session.Out, "%-*s  member\n", emailWidth, email)
	}

	return nil
}

func Remove(session api.Session, arguments []string) error {
	if len(arguments) != 2 || arguments[0] == "" {
		return errors.New("member remove takes an email address and a fleet id")
	}

	email := arguments[0]

	fleetId, err := strconv.ParseInt(arguments[1], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	fleets, err := api.FetchFleets(session)

	if err != nil {
		return err
	}

	name := ""
	found := false

	for _, fleet := range fleets {
		if fleet.Id == fleetId {
			name = fleet.Name
			found = true
		}
	}

	if !found {
		return errors.New("no such fleet")
	}

	fmt.Fprintf(session.Out, "Take away %s's access to fleet %q? [y/N] ", email, name)

	answer, _ := bufio.NewReader(session.In).ReadString('\n')

	answer = strings.ToLower(strings.TrimSpace(answer))

	if answer != "y" && answer != "yes" {
		fmt.Fprintln(session.Out, "Nothing removed.")
		return nil
	}

	request, err := api.AuthenticatedRequest(session, http.MethodDelete,
		"/fleets/"+strconv.FormatInt(fleetId, 10)+"/members/"+url.PathEscape(email), nil)

	if err != nil {
		return err
	}

	response, err := session.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your connection")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return api.ServerError(response)
	}

	fmt.Fprintf(session.Out, "Removed %s's access to fleet %q.\n", email, name)

	return nil
}
