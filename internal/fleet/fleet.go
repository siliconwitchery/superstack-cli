package fleet

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/siliconwitchery/superstack-cli/internal/api"
)

func Create(session api.Session, arguments []string) error {
	if len(arguments) != 1 || arguments[0] == "" {
		return errors.New("fleet create takes one name, quoted if it has spaces")
	}

	body, err := json.Marshal(map[string]string{"name": arguments[0]})

	if err != nil {
		return err
	}

	request, err := api.AuthenticatedRequest(session, http.MethodPost, "/fleets", bytes.NewReader(body))

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
		return api.ServerError(response)
	}

	created := struct {
		Id   int64  `json:"id"`
		Name string `json:"name"`
	}{}

	err = json.NewDecoder(response.Body).Decode(&created)

	if err != nil {
		return err
	}

	fmt.Fprintf(session.Out, "Created fleet %q with id %d.\n", created.Name, created.Id)

	return nil
}

func List(session api.Session, arguments []string) error {
	positionals, jsonOutput := api.TakeJsonFlag(arguments)

	if len(positionals) != 0 {
		return errors.New("fleet list takes no arguments")
	}

	fleets, err := api.FetchFleets(session)

	if err != nil {
		return err
	}

	if jsonOutput {
		return json.NewEncoder(session.Out).Encode(fleets)
	}

	if len(fleets) == 0 {
		fmt.Fprintln(session.Out, "No fleets yet. Create one with fleet create.")
		return nil
	}

	idWidth := len("ID")
	nameWidth := len("NAME")

	for _, fleet := range fleets {
		idWidth = max(idWidth, len(strconv.FormatInt(fleet.Id, 10)))
		nameWidth = max(nameWidth, len(fleet.Name))
	}

	fmt.Fprintf(session.Out, "%-*s  %-*s  %s\n", idWidth, "ID", nameWidth, "NAME", "ROLE")

	for _, fleet := range fleets {
		role := "member"

		if fleet.Owner {
			role = "owner"
		}

		fmt.Fprintf(session.Out, "%-*d  %-*s  %s\n", idWidth, fleet.Id, nameWidth, fleet.Name, role)
	}

	return nil
}

func Rename(session api.Session, arguments []string) error {
	if len(arguments) != 2 {
		return errors.New("fleet rename takes a fleet id and a new name, quoted if it has spaces")
	}

	fleetId, err := strconv.ParseInt(arguments[0], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	name := strings.TrimSpace(arguments[1])

	if name == "" {
		return errors.New("fleet rename takes a fleet id and a new name, quoted if it has spaces")
	}

	body, err := json.Marshal(map[string]string{"name": name})

	if err != nil {
		return err
	}

	request, err := api.AuthenticatedRequest(session, http.MethodPatch,
		"/fleets/"+strconv.FormatInt(fleetId, 10), bytes.NewReader(body))

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

	fmt.Fprintf(session.Out, "Renamed the fleet to %q.\n", name)

	return nil
}

func Transfer(session api.Session, arguments []string) error {
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

	request, err := api.AuthenticatedRequest(session, http.MethodPost,
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
		return api.ServerError(response)
	}

	fmt.Fprintf(session.Out, "Transferred the fleet to %s.\n", email)

	return nil
}

func Delete(session api.Session, arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("fleet delete takes a fleet id")
	}

	fleetId, err := strconv.ParseInt(arguments[0], 10, 64)

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

	balances, err := api.FetchBalances(session)

	if err != nil {
		return err
	}

	forfeited := ""
	forfeitUnknown := false

	for _, balance := range balances {
		if balance.Fleet != fleetId {
			continue
		}

		formatted, value, parsed := api.FormatBalance(balance)

		if !parsed {
			forfeitUnknown = true
			continue
		}

		if value > 0 {
			forfeited = formatted
		}
	}

	consequence := "It erases them all, and claiming one again means pressing its pairing button in person."

	if forfeitUnknown {
		fmt.Fprintf(session.Out, "Delete %q, release its devices, and forfeit its remaining credit? %s [y/N] ", name, consequence)
	} else if forfeited == "" {
		fmt.Fprintf(session.Out, "Delete %q and release its devices? %s [y/N] ", name, consequence)
	} else {
		fmt.Fprintf(session.Out, "Delete %q, release its devices, and forfeit its remaining %s of credit? %s [y/N] ", name, forfeited, consequence)
	}

	answer, _ := bufio.NewReader(session.In).ReadString('\n')

	answer = strings.ToLower(strings.TrimSpace(answer))

	if answer != "y" && answer != "yes" {
		fmt.Fprintln(session.Out, "Nothing deleted.")
		return nil
	}

	request, err := api.AuthenticatedRequest(session, http.MethodDelete,
		"/fleets/"+strconv.FormatInt(fleetId, 10), nil)

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

	fmt.Fprintf(session.Out, "Deleted %q.\n", name)

	return nil
}
