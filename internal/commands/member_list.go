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

func MemberList(arguments []string) error {

	jsonOutput := false

	positionals := []string{}

	for _, argument := range arguments {
		if argument == "--json" {
			jsonOutput = true
			continue
		}

		positionals = append(positionals, argument)
	}

	if len(positionals) != 1 {
		return errors.New("member list takes a fleet id")
	}

	fleetId, err := strconv.ParseInt(positionals[0], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	request, err := authenticatedRequest(http.MethodGet,
		"/fleets/"+strconv.FormatInt(fleetId, 10)+"/members", nil)

	if err != nil {
		return err
	}

	response, err := apiClient.Do(request)

	if err != nil {
		return fmt.Errorf("the server could not be reached: %w", err)
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)

	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("the server said: %s", strings.TrimSpace(string(body)))
	}

	people := struct {
		Owner   string   `json:"owner"`
		Members []string `json:"members"`
	}{}

	err = json.Unmarshal(body, &people)

	if err != nil {
		return err
	}

	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(people)
	}

	emailWidth := max(len("EMAIL"), len(people.Owner))

	for _, email := range people.Members {
		emailWidth = max(emailWidth, len(email))
	}

	fmt.Printf("%-*s  %s\n", emailWidth, "EMAIL", "ROLE")

	fmt.Printf("%-*s  owner\n", emailWidth, people.Owner)

	for _, email := range people.Members {
		fmt.Printf("%-*s  member\n", emailWidth, email)
	}

	return nil
}
