package commands

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestFleetTransfer(t *testing.T) {
	tests := []struct {
		name       string
		refusal    string
		wantOutput string
		wantError  string
	}{
		{name: "transferred", wantOutput: "Transferred the fleet to successor@example.com.\n"},
		{name: "server refusal", refusal: "the new owner has no account", wantError: "the new owner has no account"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transferredPath := ""
			transferredTo := ""
			mux := http.NewServeMux()

			mux.HandleFunc("POST /fleets/{id}/owner", func(w http.ResponseWriter, r *http.Request) {
				body := struct {
					Email string `json:"email"`
				}{}

				json.NewDecoder(r.Body).Decode(&body)
				transferredPath = r.URL.Path
				transferredTo = body.Email

				if test.refusal != "" {
					http.Error(w, test.refusal, http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusNoContent)
			})

			session, out := loggedInSession(t, mux)

			err := FleetTransfer(session, []string{"3", "successor@example.com"})

			if test.wantError != "" {
				if err == nil || err.Error() != test.wantError {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			if transferredPath != "/fleets/3/owner" || transferredTo != "successor@example.com" {
				t.Errorf("the server saw %q handed to %q, want %q handed to %q",
					transferredPath, transferredTo, "/fleets/3/owner", "successor@example.com")
			}

			if out.String() != test.wantOutput {
				t.Errorf("output = %q, want %q", out.String(), test.wantOutput)
			}
		})
	}
}

func TestFleetTransferArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantError string
	}{
		{"no arguments", nil, "takes a fleet id and an email address"},
		{"only an id", []string{"3"}, "takes a fleet id and an email address"},
		{"an empty address", []string{"3", ""}, "takes a fleet id and an email address"},
		{"a wordy id", []string{"pilot", "successor@example.com"}, "shown by fleet list"},
	}

	for _, test := range tests {
		err := FleetTransfer(Session{}, test.arguments)

		if err == nil || !strings.Contains(err.Error(), test.wantError) {
			t.Errorf("%s: error = %v, want it to mention %q", test.name, err, test.wantError)
		}
	}
}
