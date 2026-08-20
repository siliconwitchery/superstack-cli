package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

func KeyList(session Session, arguments []string) error {
	positionals, jsonOutput := takeJsonFlag(arguments)

	if len(positionals) > 1 {
		return errors.New("key list takes at most one fleet id")
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

	fetched, err := fetchKeys(session)

	if err != nil {
		return err
	}

	keys := []keyEntry{}

	for _, key := range fetched {
		if chosenFleetId == 0 || key.Fleet == chosenFleetId {
			keys = append(keys, key)
		}
	}

	if jsonOutput {
		return json.NewEncoder(session.Out).Encode(keys)
	}

	if len(keys) == 0 {
		if chosenFleetId == 0 {
			fmt.Fprintln(session.Out, "No keys yet. Create one with key create.")
		} else {
			fmt.Fprintln(session.Out, "No keys in that fleet.")
		}

		return nil
	}

	idWidth := len("ID")
	fleetIdWidth := len("FLEET")
	fleetNameWidth := len("FLEET NAME")

	for _, key := range keys {
		idWidth = max(idWidth, len(strconv.FormatInt(key.Id, 10)))
		fleetIdWidth = max(fleetIdWidth, len(strconv.FormatInt(key.Fleet, 10)))
		fleetNameWidth = max(fleetNameWidth, len(fleetNames[key.Fleet]))
	}

	fmt.Fprintf(session.Out, "%-*s  %-*s  %-*s  %-8s  %s\n",
		idWidth, "ID", fleetIdWidth, "FLEET", fleetNameWidth, "FLEET NAME", "KEY", "LABEL")

	for _, key := range keys {
		fmt.Fprintf(session.Out, "%-*d  %-*d  %-*s  ...%s  %s\n",
			idWidth, key.Id, fleetIdWidth, key.Fleet, fleetNameWidth, fleetNames[key.Fleet], key.Suffix, key.Label)
	}

	return nil
}
