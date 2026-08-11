package commands

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
)

func answerOnStdin(t *testing.T, answer string) {
	t.Helper()

	readEnd, writeEnd, err := os.Pipe()

	if err != nil {
		t.Fatal(err)
	}

	originalStdin := os.Stdin

	os.Stdin = readEnd

	t.Cleanup(func() { os.Stdin = originalStdin })

	if answer != "" {
		_, err = writeEnd.WriteString(answer)

		if err != nil {
			t.Fatal(err)
		}
	}

	writeEnd.Close()
}

func TestFleetDelete(t *testing.T) {
	tests := []struct {
		name        string
		answer      string
		wantDeleted bool
	}{
		{"confirmed with y", "y\n", true},
		{"confirmed with yes", "YES\n", true},
		{"declined with n", "n\n", false},
		{"declined by default", "\n", false},
		{"closed input", "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deletedPath := ""

			mux := http.NewServeMux()

			mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `[{"id":3,"name":"pilot","owner":true}]`)
			})

			mux.HandleFunc("DELETE /fleets/{id}", func(w http.ResponseWriter, r *http.Request) {
				deletedPath = r.URL.Path

				w.WriteHeader(http.StatusNoContent)
			})

			loggedInTestServer(t, mux)

			answerOnStdin(t, test.answer)

			err := FleetDelete([]string{"3"})

			if err != nil {
				t.Fatal(err)
			}

			if test.wantDeleted && deletedPath != "/fleets/3" {
				t.Errorf("the server saw %q deleted, want %q", deletedPath, "/fleets/3")
			}

			if !test.wantDeleted && deletedPath != "" {
				t.Errorf("the server saw %q deleted although the confirmation was declined", deletedPath)
			}
		})
	}
}

func TestFleetDeleteUnknownFleet(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /fleets", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[]`)
	})

	loggedInTestServer(t, mux)

	err := FleetDelete([]string{"9"})

	if err == nil || !strings.Contains(err.Error(), "no such fleet") {
		t.Fatalf("error = %v, want no such fleet", err)
	}
}

func TestFleetDeleteArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantError string
	}{
		{"no arguments", nil, "takes a fleet id"},
		{"two arguments", []string{"3", "4"}, "takes a fleet id"},
		{"a wordy id", []string{"pilot"}, "shown by fleet list"},
	}

	for _, test := range tests {
		err := FleetDelete(test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}
