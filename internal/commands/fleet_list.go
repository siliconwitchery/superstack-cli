package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

func FleetList(session Session, arguments []string) error {
	positionals, jsonOutput := takeJsonFlag(arguments)

	if len(positionals) != 0 {
		return errors.New("fleet list takes no arguments")
	}

	fleets, err := fetchFleets(session)

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
