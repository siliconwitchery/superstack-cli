package commands

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func FleetDelete(session Session, arguments []string) error {

	if len(arguments) != 1 {
		return errors.New("fleet delete takes a fleet id")
	}

	fleetId, err := strconv.ParseInt(arguments[0], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	fleets, err := fetchFleets(session)

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

	balances, err := fetchBalances(session)

	if err != nil {
		return err
	}

	forfeited := ""

	for _, balance := range balances {
		if balance.Fleet != fleetId {
			continue
		}

		value, err := strconv.ParseFloat(balance.Balance, 64)

		if err == nil && value > 0 {
			forfeited = formatBalance(balance)
		}
	}

	consequence := "It erases them all, and claiming one again means pressing its button in person."

	if forfeited == "" {
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

	request, err := authenticatedRequest(session, http.MethodDelete,
		"/fleets/"+strconv.FormatInt(fleetId, 10), nil)

	if err != nil {
		return err
	}

	response, err := session.Client.Do(request)

	if err != nil {
		return fmt.Errorf("the server could not be reached: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return serverError(response)
	}

	fmt.Fprintf(session.Out, "Deleted %q.\n", name)

	return nil
}
