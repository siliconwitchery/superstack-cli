package commands

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestFetchFleetsNotLoggedIn(t *testing.T) {
	isolateKeyStorage(t)

	_, err := fetchFleets(Session{})

	if err == nil || !strings.Contains(err.Error(), "not logged in") {
		t.Fatalf("error = %v, want the not-logged-in hint", err)
	}
}

func TestFetchFleetsEmptyKeyFile(t *testing.T) {
	session, _ := loggedInSession(t, http.NotFoundHandler())

	path, err := keyPath()

	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(path, []byte("  \n"), 0o600)

	if err != nil {
		t.Fatal(err)
	}

	_, err = fetchFleets(session)

	if err == nil || !strings.Contains(err.Error(), "not logged in") {
		t.Fatalf("error = %v, want the not-logged-in hint for an empty key file", err)
	}
}
