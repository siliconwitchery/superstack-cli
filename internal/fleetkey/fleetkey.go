package fleetkey

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

func Create(invocation api.Invocation, arguments []string) error {
	if len(arguments) != 2 || arguments[1] == "" {
		return errors.New("key create takes a fleet id and a label, quoted if it has spaces")
	}

	fleetId, err := strconv.ParseInt(arguments[0], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	body, err := json.Marshal(map[string]string{"label": arguments[1]})

	if err != nil {
		return err
	}

	request, err := api.AuthenticatedRequest(invocation, http.MethodPost,
		"/fleets/"+strconv.FormatInt(fleetId, 10)+"/fleet-keys", bytes.NewReader(body))

	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := invocation.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your internet access")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return api.ServerError(response)
	}

	created := struct {
		Id       int64  `json:"id"`
		FleetKey string `json:"fleet_key"`
	}{}

	err = api.Decode(response, &created)

	if err != nil {
		return err
	}

	if created.FleetKey == "" {
		return errors.New("the key was not created, try again")
	}

	fmt.Fprintf(invocation.Out, "Created key %d.\n\n  %s\n\nAnyone holding it can send data to the fleet, and you will not see it again.\n", created.Id, api.Printable(created.FleetKey))

	return nil
}

func List(invocation api.Invocation, arguments []string) error {
	positionals, jsonOutput := api.TakeJsonFlag(arguments)

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

	fleets, err := api.FetchFleets(invocation)

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

	fetched, err := api.FetchFleetKeys(invocation)

	if err != nil {
		return err
	}

	fleetKeys := []api.FleetKeyEntry{}

	for _, fleetKey := range fetched {
		if chosenFleetId == 0 || fleetKey.FleetId == chosenFleetId {
			fleetKeys = append(fleetKeys, fleetKey)
		}
	}

	if jsonOutput {
		err = json.NewEncoder(invocation.Out).Encode(fleetKeys)

		return err
	}

	if len(fleetKeys) == 0 {
		if chosenFleetId == 0 {
			fmt.Fprintln(invocation.Out, "No keys yet. Create one with key create.")
		} else {
			fmt.Fprintln(invocation.Out, "No keys on that fleet yet.")
		}

		return nil
	}

	idWidth := len("ID")
	fleetIdWidth := len("FLEET")
	fleetNameWidth := len("FLEET NAME")
	fleetKeyWidth := len("KEY")
	fleetNameValues := make([]string, len(fleetKeys))
	suffixValues := make([]string, len(fleetKeys))
	labelValues := make([]string, len(fleetKeys))

	for index, fleetKey := range fleetKeys {
		fleetName, known := fleetNames[fleetKey.FleetId]

		if !known {
			fleetName = "-"
		}

		fleetNameValues[index] = api.Printable(fleetName)
		suffixValues[index] = api.Printable(fleetKey.FleetKeySuffix)
		labelValues[index] = api.Printable(fleetKey.Label)
		idWidth = max(idWidth, len(strconv.FormatInt(fleetKey.Id, 10)))
		fleetIdWidth = max(fleetIdWidth, len(strconv.FormatInt(fleetKey.FleetId, 10)))
		fleetNameWidth = max(fleetNameWidth, len(fleetNameValues[index]))
	}

	fmt.Fprintf(invocation.Out, "%-*s  %-*s  %-*s  %-*s  %s\n",
		idWidth, "ID", fleetIdWidth, "FLEET", fleetNameWidth, "FLEET NAME", fleetKeyWidth, "KEY", "LABEL")

	for index, fleetKey := range fleetKeys {
		fmt.Fprintf(invocation.Out, "%-*d  %-*d  %-*s  %-*s  %s\n",
			idWidth, fleetKey.Id, fleetIdWidth, fleetKey.FleetId, fleetNameWidth, fleetNameValues[index],
			fleetKeyWidth, "..."+suffixValues[index], labelValues[index])
	}

	return nil
}

func Revoke(invocation api.Invocation, arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("key revoke takes a key id")
	}

	fleetKeyId, err := strconv.ParseInt(arguments[0], 10, 64)

	if err != nil || fleetKeyId < 1 {
		return errors.New("the key id is the number shown by key list")
	}

	fleetKeys, err := api.FetchFleetKeys(invocation)

	if err != nil {
		return err
	}

	label := ""
	found := false

	for _, fleetKey := range fleetKeys {
		if fleetKey.Id == fleetKeyId {
			label = fleetKey.Label
			found = true
		}
	}

	if !found {
		return errors.New("no such key")
	}

	fmt.Fprintf(invocation.Out, "Revoke key %q? Anything still using it stops reaching the fleet. [y/N] ", label)

	answer, _ := bufio.NewReader(invocation.In).ReadString('\n')

	answer = strings.ToLower(strings.TrimSpace(answer))

	if answer != "y" && answer != "yes" {
		fmt.Fprintln(invocation.Out, "Nothing revoked.")
		return nil
	}

	request, err := api.AuthenticatedRequest(invocation, http.MethodDelete,
		"/fleet-keys/"+strconv.FormatInt(fleetKeyId, 10), nil)

	if err != nil {
		return err
	}

	response, err := invocation.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your internet access")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return api.ServerError(response)
	}

	fmt.Fprintf(invocation.Out, "Revoked key %q.\n", label)

	return nil
}
