package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

func FleetList(arguments []string) error {

	jsonOutput := false

	for _, argument := range arguments {
		if argument != "--json" {
			return fmt.Errorf("fleet list takes no arguments, only --json")
		}

		jsonOutput = true
	}

	fleets, err := fetchFleets()

	if err != nil {
		return err
	}

	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(fleets)
	}

	if len(fleets) == 0 {
		fmt.Println("No fleets yet. Create one with fleet create.")
		return nil
	}

	idWidth := 0
	nameWidth := 0

	for _, fleet := range fleets {
		idWidth = max(idWidth, len(strconv.FormatInt(fleet.Id, 10)))
		nameWidth = max(nameWidth, len(fleet.Name))
	}

	for _, fleet := range fleets {
		role := "member"

		if fleet.Owner {
			role = "owner"
		}

		fmt.Printf("%-*d  %-*s  %s\n", idWidth, fleet.Id, nameWidth, fleet.Name, role)
	}

	return nil
}
