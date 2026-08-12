package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func KeyCreate(arguments []string) error {

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

	request, err := authenticatedRequest(http.MethodPost,
		"/fleets/"+strconv.FormatInt(fleetId, 10)+"/keys", bytes.NewReader(body))

	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := apiClient.Do(request)

	if err != nil {
		return fmt.Errorf("the server could not be reached: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))

		return fmt.Errorf("the server said: %s", strings.TrimSpace(string(message)))
	}

	created := struct {
		Id  int64  `json:"id"`
		Key string `json:"key"`
	}{}

	err = json.NewDecoder(response.Body).Decode(&created)

	if err != nil {
		return err
	}

	fmt.Printf("Created key %d.\n\n  %s\n\nAnyone holding it can send data to the fleet, and it is shown only this once.\n", created.Id, created.Key)

	return nil
}
