package commands

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

func AccountDelete(session Session, arguments []string) error {
	if len(arguments) != 0 {
		return errors.New("account delete takes no arguments")
	}

	fmt.Fprint(session.Out, "Delete your account, its logins, and your access to every fleet? This cannot be undone. [y/N] ")

	answer, _ := bufio.NewReader(session.In).ReadString('\n')

	answer = strings.ToLower(strings.TrimSpace(answer))

	if answer != "y" && answer != "yes" {
		fmt.Fprintln(session.Out, "Nothing deleted.")
		return nil
	}

	request, err := authenticatedRequest(session, http.MethodDelete, "/account", nil)

	if err != nil {
		return err
	}

	response, err := session.Client.Do(request)

	if err != nil {
		return errors.New("the server could not be reached, check your connection")
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusNoContent {
		return serverError(response)
	}

	// Remove the stored login
	path, err := keyPath()

	if err != nil {
		return err
	}

	err = os.Remove(path)

	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	fmt.Fprintln(session.Out, "Account deleted.")

	return nil
}
