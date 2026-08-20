package commands

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func KeyRevoke(session Session, arguments []string) error {

	if len(arguments) != 1 {
		return errors.New("key revoke takes a key id")
	}

	keyId, err := strconv.ParseInt(arguments[0], 10, 64)

	if err != nil || keyId < 1 {
		return errors.New("the key id is the number shown by key list")
	}

	keys, err := fetchKeys(session)

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

	fmt.Fprintf(session.Out, "Revoke %q? Anything still using it stops reaching the fleet. [y/N] ", label)

	answer, _ := bufio.NewReader(session.In).ReadString('\n')

	answer = strings.ToLower(strings.TrimSpace(answer))

	if answer != "y" && answer != "yes" {
		fmt.Fprintln(session.Out, "Nothing revoked.")
		return nil
	}

	request, err := authenticatedRequest(session, http.MethodDelete,
		"/keys/"+strconv.FormatInt(keyId, 10), nil)

	if err != nil {
		return err
	}

	response, err := session.Client.Do(request)

	if err != nil {
		return fmt.Errorf("the server could not be reached: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return serverError(response)
	}

	fmt.Fprintf(session.Out, "Revoked key %d.\n", keyId)

	return nil
}
