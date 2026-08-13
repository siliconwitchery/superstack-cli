package commands

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

func AccountDelete(arguments []string) error {

	if len(arguments) != 0 {
		return errors.New("account delete takes no arguments")
	}

	fmt.Print("Delete your account, its logins, and your access to every fleet? This cannot be undone. [y/N] ")

	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')

	answer = strings.ToLower(strings.TrimSpace(answer))

	if answer != "y" && answer != "yes" {
		fmt.Println("Nothing deleted.")
		return nil
	}

	request, err := authenticatedRequest(http.MethodDelete, "/account", nil)

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

	// The stored login died with the account, so it goes whether or not the
	// file is still there
	path, err := keyPath()

	if err != nil {
		return err
	}

	err = os.Remove(path)

	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	fmt.Println("Account deleted.")

	return nil
}
