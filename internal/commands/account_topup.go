package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func AccountTopup(arguments []string) error {

	if len(arguments) != 1 {
		return errors.New("account topup takes a fleet id")
	}

	fleetId, err := strconv.ParseInt(arguments[0], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	request, err := authenticatedRequest(http.MethodPost,
		"/fleets/"+strconv.FormatInt(fleetId, 10)+"/topup", nil)

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

	opened := struct {
		Url string `json:"url"`
	}{}

	err = json.NewDecoder(response.Body).Decode(&opened)

	if err != nil || opened.Url == "" {
		return errors.New("the payment page could not be opened, try again")
	}

	fmt.Printf("Open this link to choose an amount and pay:\n\n  %s\n\nThe credit appears on the balance once the payment completes.\n", opened.Url)

	return nil
}
