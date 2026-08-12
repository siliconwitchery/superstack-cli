package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type keyEntry struct {
	Id     int64  `json:"id"`
	Fleet  int64  `json:"fleet"`
	Label  string `json:"label"`
	Suffix string `json:"suffix"`
}

func KeyList(arguments []string) error {

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

	request, err := authenticatedRequest(http.MethodGet, "/keys", nil)

	if err != nil {
		return err
	}

	response, err := apiClient.Do(request)

	if err != nil {
		return fmt.Errorf("the server could not be reached: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))

		return fmt.Errorf("the server said: %s", strings.TrimSpace(string(message)))
	}

	fetched := []keyEntry{}

	err = json.NewDecoder(response.Body).Decode(&fetched)

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
		return json.NewEncoder(os.Stdout).Encode(keys)
	}

	if len(keys) == 0 {
		fmt.Println("No keys yet. Create one with key create.")
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

	fmt.Printf("%-*s  %-*s  %-*s  %-8s  %s\n",
		idWidth, "ID", fleetIdWidth, "FLEET", fleetNameWidth, "FLEET NAME", "KEY", "LABEL")

	for _, key := range keys {
		fmt.Printf("%-*d  %-*d  %-*s  ...%s  %s\n",
			idWidth, key.Id, fleetIdWidth, key.Fleet, fleetNameWidth, fleetNames[key.Fleet], key.Suffix, key.Label)
	}

	return nil
}
