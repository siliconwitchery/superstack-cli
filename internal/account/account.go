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
	"github.com/siliconwitchery/superstack-cli/internal/dispatch"
)

func Balance(session api.Session, arguments []string) error {
	positionals, jsonOutput := dispatch.TakeJsonFlag(arguments)

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

	fetched, err := api.FetchBalances(session)

	if err != nil {
		return err
	}

	balances := []api.BalanceEntry{}

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
		formatted, _, _ := api.FormatBalance(balance)

		fmt.Fprintf(session.Out, "%-*d  %-*s  %s\n", idWidth, balance.Fleet, nameWidth, fleetNames[balance.Fleet], formatted)
	}

	return nil
}

func Topup(session api.Session, arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("account topup takes a fleet id")
	}

	fleetId, err := strconv.ParseInt(arguments[0], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	request, err := api.AuthenticatedRequest(session, http.MethodPost,
		"/fleets/"+strconv.FormatInt(fleetId, 10)+"/topup", nil)

	if err != nil {
		return err
	}

	response, err := session.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your connection")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return api.ServerError(response)
	}

	opened := struct {
		Url string `json:"url"`
	}{}

	err = json.NewDecoder(response.Body).Decode(&opened)

	if err != nil || opened.Url == "" {
		return errors.New("could not open the top-up page, try again")
	}

	fmt.Fprintf(session.Out, "Open this link to choose an amount and pay:\n\n  %s\n\nThe credit appears on the balance once the top-up completes.\nPress enter to open the browser.\n", opened.Url)

	_, err = bufio.NewReader(session.In).ReadString('\n')

	if err != nil {
		return nil
	}

	session.OpenBrowser(opened.Url)

	return nil
}

func Delete(session api.Session, arguments []string) error {
	if len(arguments) != 0 {
		return errors.New("account delete takes no arguments")
	}

	fmt.Fprint(session.Out, "Delete your account, its logins, and your access to every fleet? This cannot be undone. [y/N] ")

	answer, _ := bufio.NewReader(session.In).ReadString('\n')

	answer = strings.ToLower(strings.TrimSpace(answer))

	if answer != "y" && answer != "yes" {
		fmt.Fprintln(session.Out, "Nothing deleted.")
		return nil
	}

	request, err := api.AuthenticatedRequest(session, http.MethodDelete, "/account", nil)

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

	path, err := api.KeyPath()

	if err != nil {
		return err
	}

	err = os.Remove(path)

	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	fmt.Fprintln(session.Out, "Account deleted.")

	return nil
}
