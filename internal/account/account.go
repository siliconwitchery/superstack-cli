package account

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/siliconwitchery/superstack-cli/internal/api"
)

func Balance(invocation api.Invocation, arguments []string) error {
	positionals, jsonOutput := api.TakeJsonFlag(arguments)

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

	fetched, err := api.FetchBalances(invocation)

	if err != nil {
		return err
	}

	balances := []api.BalanceEntry{}

	for _, balance := range fetched {
		if chosenFleetId == 0 || balance.FleetId == chosenFleetId {
			balances = append(balances, balance)
		}
	}

	if jsonOutput {
		err = json.NewEncoder(invocation.Out).Encode(balances)

		return err
	}

	if len(balances) == 0 {
		if chosenFleetId == 0 {
			fmt.Fprintln(invocation.Out, "No fleets yet. Create one with fleet create.")
		} else {
			fmt.Fprintln(invocation.Out, "No credit on that fleet yet.")
		}

		return nil
	}

	idWidth := len("ID")
	nameWidth := len("NAME")
	nameValues := make([]string, len(balances))
	amountValues := make([]string, len(balances))

	for index, balance := range balances {
		name, known := fleetNames[balance.FleetId]

		if !known {
			name = "-"
		}

		formatted, _, _ := api.FormatBalance(balance)

		nameValues[index] = api.Printable(name)
		amountValues[index] = api.Printable(formatted)
		idWidth = max(idWidth, len(strconv.FormatInt(balance.FleetId, 10)))
		nameWidth = max(nameWidth, len(nameValues[index]))
	}

	fmt.Fprintf(invocation.Out, "%-*s  %-*s  %s\n", idWidth, "ID", nameWidth, "NAME", "BALANCE")

	for index, balance := range balances {
		fmt.Fprintf(invocation.Out, "%-*d  %-*s  %s\n", idWidth, balance.FleetId, nameWidth, nameValues[index], amountValues[index])
	}

	return nil
}

func TopUp(invocation api.Invocation, arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("account topup takes a fleet id")
	}

	fleetId, err := strconv.ParseInt(arguments[0], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	request, err := api.AuthenticatedRequest(invocation, http.MethodPost,
		"/fleets/"+strconv.FormatInt(fleetId, 10)+"/top-ups", nil)

	if err != nil {
		return err
	}

	response, err := invocation.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your internet access")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return api.ServerError(response)
	}

	opened := struct {
		Url string `json:"url"`
	}{}

	err = api.Decode(response, &opened)

	if err != nil || opened.Url == "" {
		return errors.New("could not open the top-up page, try again")
	}

	fmt.Fprintf(invocation.Out, "Open this page to choose an amount and pay:\n\n  %s\n\nThe credit appears on the balance once the top-up completes.\nPress enter to open the browser.\n", api.Printable(opened.Url))

	_, err = bufio.NewReader(invocation.In).ReadString('\n')

	if err != nil {
		return nil
	}

	invocation.OpenBrowser(opened.Url)

	return nil
}

func Delete(invocation api.Invocation, arguments []string) error {
	if len(arguments) != 0 {
		return errors.New("account delete takes no arguments")
	}

	request, err := api.AuthenticatedRequest(invocation, http.MethodGet, "/fleets", nil)

	if err != nil {
		return err
	}

	response, err := invocation.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your internet access")
	}

	if response.StatusCode != http.StatusOK {
		refusal := api.ServerError(response)

		response.Body.Close()

		return refusal
	}

	response.Body.Close()

	fmt.Fprint(invocation.Out, "Delete your account, its logins, and your access to every fleet? This cannot be undone. [y/N] ")

	answer, _ := bufio.NewReader(invocation.In).ReadString('\n')

	answer = strings.ToLower(strings.TrimSpace(answer))

	if answer != "y" && answer != "yes" {
		fmt.Fprintln(invocation.Out, "Nothing deleted.")
		return nil
	}

	request, err = api.AuthenticatedRequest(invocation, http.MethodDelete, "/account", nil)

	if err != nil {
		return err
	}

	response, err = invocation.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your internet access")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return api.ServerError(response)
	}

	fmt.Fprintln(invocation.Out, "Account deleted.")

	path, err := api.LoginKeyPath()

	if err != nil {
		return err
	}

	err = os.Remove(path)

	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("the login stored at %s is no longer valid but could not be removed, delete it yourself", path)
	}

	return nil
}
