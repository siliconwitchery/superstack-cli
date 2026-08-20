package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

func AccountBalance(session Session, arguments []string) error {
	positionals, jsonOutput := takeJsonFlag(arguments)

	if len(positionals) > 1 {
		return errors.New("account balance takes at most one fleet id")
	}

	chosenFleetId := int64(0)

	if len(positionals) == 1 {
		parsed, err := strconv.ParseInt(positionals[0], 10, 64)

		if err != nil || parsed < 1 {
			return errors.New("the fleet id is the number shown by fleet list")
		}

		chosenFleetId = parsed
	}

	fleets, err := fetchFleets(session)

	if err != nil {
		return err
	}

	fleetNames := map[int64]string{}

	for _, fleet := range fleets {
		fleetNames[fleet.Id] = fleet.Name
	}

	if chosenFleetId != 0 {
		if _, found := fleetNames[chosenFleetId]; !found {
			return errors.New("no such fleet")
		}
	}

	fetched, err := fetchBalances(session)

	if err != nil {
		return err
	}

	balances := []balanceEntry{}

	for _, balance := range fetched {
		if chosenFleetId == 0 || balance.Fleet == chosenFleetId {
			balances = append(balances, balance)
		}
	}

	if jsonOutput {
		return json.NewEncoder(session.Out).Encode(balances)
	}

	if len(balances) == 0 {
		if chosenFleetId == 0 {
			fmt.Fprintln(session.Out, "No fleets yet. Create one with fleet create.")
		} else {
			fmt.Fprintln(session.Out, "No credit on that fleet yet.")
		}

		return nil
	}

	idWidth := len("ID")
	nameWidth := len("NAME")

	for _, balance := range balances {
		idWidth = max(idWidth, len(strconv.FormatInt(balance.Fleet, 10)))
		nameWidth = max(nameWidth, len(fleetNames[balance.Fleet]))
	}

	fmt.Fprintf(session.Out, "%-*s  %-*s  %s\n", idWidth, "ID", nameWidth, "NAME", "BALANCE")

	for _, balance := range balances {
		formatted, _, _ := formatBalance(balance)

		fmt.Fprintf(session.Out, "%-*d  %-*s  %s\n", idWidth, balance.Fleet, nameWidth, fleetNames[balance.Fleet], formatted)
	}

	return nil
}
