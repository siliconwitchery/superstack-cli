package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func FleetCreate(arguments []string) error {

	if len(arguments) != 1 || arguments[0] == "" {
		return errors.New("fleet create takes one name, quoted if it has spaces")
	}

	body, err := json.Marshal(map[string]string{"name": arguments[0]})

	if err != nil {
		return err
	}

	request, err := authenticatedRequest(http.MethodPost, "/fleets", bytes.NewReader(body))

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
		Id   int64  `json:"id"`
		Name string `json:"name"`
	}{}

	err = json.NewDecoder(response.Body).Decode(&created)

	if err != nil {
		return err
	}

	fmt.Printf("Created fleet %q with id %d.\n", created.Name, created.Id)

	return nil
}
