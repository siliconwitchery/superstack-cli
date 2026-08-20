package commands

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

func AccountTopup(session Session, arguments []string) error {
	if len(arguments) != 1 {
		return errors.New("account topup takes a fleet id")
	}

	fleetId, err := strconv.ParseInt(arguments[0], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	request, err := authenticatedRequest(session, http.MethodPost,
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
		return serverError(response)
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
