package commands

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func KeyRevoke(arguments []string) error {

	if len(arguments) != 1 {
		return errors.New("key revoke takes a key id")
	}

	keyId, err := strconv.ParseInt(arguments[0], 10, 64)

	if err != nil || keyId < 1 {
		return errors.New("the key id is the number shown by key list")
	}

	request, err := authenticatedRequest(http.MethodDelete,
		"/keys/"+strconv.FormatInt(keyId, 10), nil)

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

	fmt.Printf("Revoked key %d.\n", keyId)

	return nil
}
