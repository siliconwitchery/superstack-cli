package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

func Logout(session Session, arguments []string) error {

	if len(arguments) != 0 {
		return errors.New("logout takes no arguments")
	}

	path, err := keyPath()

	if err != nil {
		return err
	}

	keyBytes, err := os.ReadFile(path)

	if errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintln(session.Out, "Not logged in.")
		return nil
	}

	if err != nil {
		return err
	}

	// Revoke on the server first, keeping the key on any failure so another
	// logout can retry; a forgotten key can never be revoked
	revokeRequest, err := apiRequest(session, http.MethodPost, "/logout", nil)

	if err != nil {
		return err
	}

	revokeRequest.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(keyBytes)))

	revokeResponse, err := session.Client.Do(revokeRequest)

	if err != nil {
		return fmt.Errorf("you are still logged in: the server could not be reached: %w", err)
	}

	defer revokeResponse.Body.Close()

	if revokeResponse.StatusCode != http.StatusNoContent {
		return fmt.Errorf("you are still logged in: %w", serverError(revokeResponse))
	}

	err = os.Remove(path)

	if err != nil {
		return err
	}

	fmt.Fprintln(session.Out, "Logged out.")

	return nil
}
