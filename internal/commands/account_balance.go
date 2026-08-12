package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
)

func AccountBalance(arguments []string) error {

	jsonOutput := false

	positionals := []string{}

	for _, argument := range arguments {
		if argument == "--json" {
			jsonOutput = true
			continue
		}

		positionals = append(positionals, argument)
	}

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

	fleets, err := fetchFleets()

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

	fetched, err := fetchBalances()

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
		return json.NewEncoder(os.Stdout).Encode(balances)
	}

	if len(balances) == 0 {
		fmt.Println("No fleets yet. Create one with fleet create.")
		return nil
	}

	idWidth := len("ID")
	nameWidth := len("NAME")

	for _, balance := range balances {
		idWidth = max(idWidth, len(strconv.FormatInt(balance.Fleet, 10)))
		nameWidth = max(nameWidth, len(fleetNames[balance.Fleet]))
	}

	fmt.Printf("%-*s  %-*s  %s\n", idWidth, "ID", nameWidth, "NAME", "BALANCE")

	for _, balance := range balances {
		fmt.Printf("%-*d  %-*s  %s\n", idWidth, balance.Fleet, nameWidth, fleetNames[balance.Fleet], formatBalance(balance))
	}

	return nil
}
