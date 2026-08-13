package commands

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func MemberRemove(arguments []string) error {

	if len(arguments) != 2 || arguments[0] == "" {
		return errors.New("member remove takes an email address and a fleet id")
	}

	email := arguments[0]

	fleetId, err := strconv.ParseInt(arguments[1], 10, 64)

	if err != nil || fleetId < 1 {
		return errors.New("the fleet id is the number shown by fleet list")
	}

	fleets, err := fetchFleets()

	if err != nil {
		return err
	}

	name := ""
	found := false

	for _, fleet := range fleets {
		if fleet.Id == fleetId {
			name = fleet.Name
			found = true
		}
	}

	if !found {
		return errors.New("no such fleet")
	}

	fmt.Printf("Take away %s's access to %q? [y/N] ", email, name)

	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')

	answer = strings.ToLower(strings.TrimSpace(answer))

	if answer != "y" && answer != "yes" {
		fmt.Println("Nothing changed.")
		return nil
	}

	request, err := authenticatedRequest(http.MethodDelete,
		"/fleets/"+strconv.FormatInt(fleetId, 10)+"/members/"+url.PathEscape(email), nil)

	if err != nil {
		return err
	}

	response, err := apiClient.Do(request)

	if err != nil {
		return fmt.Errorf("the server could not be reached: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))

		return fmt.Errorf("the server said: %s", strings.TrimSpace(string(message)))
	}

	fmt.Printf("Removed access for %s.\n", email)

	return nil
}
