package key

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

	request, err := api.AuthenticatedRequest(session, http.MethodPost,
		"/fleets/"+strconv.FormatInt(fleetId, 10)+"/keys", bytes.NewReader(body))

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
		Id  int64  `json:"id"`
		Key string `json:"key"`
	}{}

	err = json.NewDecoder(response.Body).Decode(&created)

	if err != nil {
		return err
	}

	fmt.Fprintf(session.Out, "Created fleet key %d.\n\n  %s\n\nAnyone holding it can send data to the fleet, and you will not see it again.\n", created.Id, created.Key)

	return nil
}

func List(session api.Session, arguments []string) error {
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

	fleets, err := api.FetchFleets(session)

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

	fetched, err := api.FetchKeys(session)

	if err != nil {
		return err
	}

	keys := []api.KeyEntry{}

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
			fmt.Fprintln(session.Out, "No fleet keys yet. Create one with key create.")
		} else {
			fmt.Fprintln(session.Out, "No fleet keys on that fleet yet.")
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

func Revoke(session api.Session, arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("key revoke takes a key id")
	}

	keyId, err := strconv.ParseInt(arguments[0], 10, 64)

	if err != nil || keyId < 1 {
		return errors.New("the key id is the number shown by key list")
	}

	keys, err := api.FetchKeys(session)

	if err != nil {
		return err
	}

	label := ""
	found := false

	for _, key := range keys {
		if key.Id == keyId {
			label = key.Label
			found = true
		}
	}

	if !found {
		return errors.New("no such key")
	}

	fmt.Fprintf(session.Out, "Revoke fleet key %q? Anything still using it stops reaching the fleet. [y/N] ", label)

	answer, _ := bufio.NewReader(session.In).ReadString('\n')

	answer = strings.ToLower(strings.TrimSpace(answer))

	if answer != "y" && answer != "yes" {
		fmt.Fprintln(session.Out, "Nothing revoked.")
		return nil
	}

	request, err := api.AuthenticatedRequest(session, http.MethodDelete,
		"/keys/"+strconv.FormatInt(keyId, 10), nil)

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

	fmt.Fprintf(session.Out, "Revoked fleet key %q.\n", label)

	return nil
}
