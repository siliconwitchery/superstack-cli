package commands

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestFetchFleetsFailures(t *testing.T) {
	tests := []struct {
		name      string
		loggedIn  bool
		storedKey string
		status    int
		body      string
		wantError string
	}{
		{name: "not logged in", wantError: "not logged in"},
		{name: "empty key file", loggedIn: true, storedKey: "  \n", wantError: "not logged in"},
		{name: "server refusal", loggedIn: true, status: http.StatusServiceUnavailable, body: "fleets unavailable", wantError: "fleets unavailable"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateKeyStorage(t)

			session := Session{}

			if test.loggedIn {
				mux := http.NewServeMux()
				mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
					if test.status != 0 {
						w.WriteHeader(test.status)
					}

					fmt.Fprint(w, test.body)
				})

				session, _ = loggedInSession(t, mux)

				if test.storedKey != "" {
					path, err := keyPath()

					if err != nil {
						t.Fatal(err)
					}

					err = os.WriteFile(path, []byte(test.storedKey), 0o600)

					if err != nil {
						t.Fatal(err)
					}
				}
			}

			_, err := fetchFleets(session)

			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want it to mention %q", err, test.wantError)
			}
		})
	}
}
